package tracker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robertlestak/stackver/pkg/utils"
	log "github.com/sirupsen/logrus"
)

type OCITracker struct {
	uri              string
	acceptPrerelease bool
}

type OCITagsResponse struct {
	Tags []string `json:"tags"`
}

type OCIArtifact struct {
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

type OCITokenResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
}

var ociBearerTokenCache = struct {
	sync.Mutex
	tokens map[string]string
}{
	tokens: map[string]string{},
}

func (t *OCITracker) URI() string {
	return t.uri
}

func (t *OCITracker) Link() string {
	return fmt.Sprintf("https://%s", t.uri)
}

func (t *OCITracker) GetStatus(currentVersion string) (ServiceStatus, error) {
	return t.GetStatusWithOffset(currentVersion, 0)
}

func (t *OCITracker) GetStatusWithOffset(currentVersion string, offset int) (ServiceStatus, error) {
	l := log.WithFields(log.Fields{
		"tracker": "oci",
		"uri":     t.uri,
	})
	l.Debug("getting status")

	// Parse registry and repository from URI
	parts := strings.SplitN(t.uri, "/", 2)
	if len(parts) != 2 {
		return ServiceStatus{}, fmt.Errorf("invalid OCI URI format: %s", t.uri)
	}
	registry := parts[0]
	repository := parts[1]

	var tags []string
	var err error

	// Try OCI Distribution API first
	var distributionErr error
	tags, err = t.getTagsFromDistributionAPI(registry, repository)
	if err != nil {
		distributionErr = err
		l.Debug("Distribution API failed, trying registry-specific API")
		// Fallback to registry-specific API (Harbor, etc.)
		tags, err = t.getTagsFromRegistryAPI(registry, repository)
		if err != nil {
			return ServiceStatus{}, fmt.Errorf("failed to get tags from distribution API: %v; registry API: %v", distributionErr, err)
		}
	}

	if len(tags) == 0 {
		return ServiceStatus{}, fmt.Errorf("no tags found")
	}

	// Get version at offset
	targetVersion := utils.GetVersionAtOffset(tags, offset, t.acceptPrerelease)
	if targetVersion == "" {
		return ServiceStatus{}, fmt.Errorf("no suitable version found")
	}

	stat := ServiceStatus{
		LatestVersion: utils.TrimVersionPrefix(targetVersion),
		Link:          t.Link(),
	}
	stat.CalculateStatus(currentVersion)
	l = l.WithField("stat", stat)
	l.Debug("got status")
	return stat, nil
}

func (t *OCITracker) getTagsFromDistributionAPI(registry, repository string) ([]string, error) {
	endpoint := fmt.Sprintf("https://%s/v2/%s/tags/list", registry, repository)

	resp, err := t.doRegistryRequest(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("distribution API returned status %d", resp.StatusCode)
	}

	var tagsResp OCITagsResponse
	err = json.NewDecoder(resp.Body).Decode(&tagsResp)
	if err != nil {
		return nil, err
	}

	return tagsResp.Tags, nil
}

func (t *OCITracker) doRegistryRequest(endpoint string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := doRegistryRequestWithRetries(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	authHeader := resp.Header.Get("WWW-Authenticate")
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	token, err := t.getBearerToken(authHeader)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, fmt.Errorf("registry returned 401 without a bearer token challenge")
	}

	req, err = http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	return doRegistryRequestWithRetries(req)
}

func doRegistryRequestWithRetries(req *http.Request) (*http.Response, error) {
	const maxAttempts = 5

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := http.DefaultClient.Do(req.Clone(req.Context()))
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusTooManyRequests || attempt == maxAttempts {
			return resp, nil
		}

		wait := retryAfter(resp.Header.Get("Retry-After"), attempt)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		time.Sleep(wait)
	}

	return nil, fmt.Errorf("registry request retry loop exhausted")
}

func retryAfter(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if retryTime, err := http.ParseTime(header); err == nil {
		wait := time.Until(retryTime)
		if wait > 0 {
			return wait
		}
	}

	wait := time.Duration(1<<uint(attempt-1)) * time.Second
	if wait > 8*time.Second {
		return 8 * time.Second
	}
	return wait
}

func (t *OCITracker) getBearerToken(authHeader string) (string, error) {
	scheme, params, err := parseWWWAuthenticate(authHeader)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(scheme, "Bearer") {
		return "", fmt.Errorf("unsupported registry auth scheme %q", scheme)
	}

	realm := params["realm"]
	if realm == "" {
		return "", fmt.Errorf("registry bearer challenge missing realm")
	}

	cacheKey := authHeader
	ociBearerTokenCache.Lock()
	if token := ociBearerTokenCache.tokens[cacheKey]; token != "" {
		ociBearerTokenCache.Unlock()
		return token, nil
	}
	defer ociBearerTokenCache.Unlock()

	tokenURL, err := url.Parse(realm)
	if err != nil {
		return "", err
	}
	q := tokenURL.Query()
	if service := params["service"]; service != "" {
		q.Set("service", service)
	}
	if scope := params["scope"]; scope != "" {
		q.Set("scope", scope)
	}
	tokenURL.RawQuery = q.Encode()

	resp, err := http.Get(tokenURL.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned status %d", resp.StatusCode)
	}

	var tokenResp OCITokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}
	if tokenResp.Token != "" {
		ociBearerTokenCache.tokens[cacheKey] = tokenResp.Token
		return tokenResp.Token, nil
	}
	ociBearerTokenCache.tokens[cacheKey] = tokenResp.AccessToken
	return tokenResp.AccessToken, nil
}

func parseWWWAuthenticate(header string) (string, map[string]string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", nil, fmt.Errorf("missing WWW-Authenticate header")
	}

	parts := strings.SplitN(header, " ", 2)
	scheme := parts[0]
	params := map[string]string{}
	if len(parts) == 1 {
		return scheme, params, nil
	}

	for _, part := range splitAuthParams(parts[1]) {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		params[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(value), `"`)
	}

	return scheme, params, nil
}

func splitAuthParams(raw string) []string {
	var params []string
	start := 0
	inQuotes := false

	for i, r := range raw {
		switch r {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				params = append(params, raw[start:i])
				start = i + 1
			}
		}
	}

	params = append(params, raw[start:])
	return params
}

func (t *OCITracker) getTagsFromRegistryAPI(registry, repository string) ([]string, error) {
	// Try common registry API patterns
	endpoints := []string{
		fmt.Sprintf("https://%s/api/v2.0/projects/%s/repositories/%s/artifacts", registry, getProject(repository), url.QueryEscape(getRepo(repository))),
		fmt.Sprintf("https://%s/v2/%s/tags/list", registry, repository),
	}

	for _, endpoint := range endpoints {
		tags, err := t.tryRegistryEndpoint(endpoint)
		if err == nil && len(tags) > 0 {
			return tags, nil
		}
	}

	return nil, fmt.Errorf("no working registry API found")
}

func (t *OCITracker) tryRegistryEndpoint(endpoint string) ([]string, error) {
	resp, err := t.doRegistryRequest(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Try to parse as Harbor-style artifacts response
	var artifacts []OCIArtifact
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&artifacts); err == nil {
		var tags []string
		for _, artifact := range artifacts {
			for _, tag := range artifact.Tags {
				tags = append(tags, tag.Name)
			}
		}
		return tags, nil
	}

	// Try to parse as standard OCI tags response
	var tagsResp OCITagsResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&tagsResp); err == nil {
		return tagsResp.Tags, nil
	}

	return nil, fmt.Errorf("unable to parse response")
}

func getProject(repository string) string {
	parts := strings.Split(repository, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return repository
}

func getRepo(repository string) string {
	parts := strings.SplitN(repository, "/", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return repository
}

func (t *OCITracker) SetAcceptPrerelease(accept bool) {
	t.acceptPrerelease = accept
}
