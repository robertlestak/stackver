package tracker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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
	tags, err = t.getTagsFromDistributionAPI(registry, repository)
	if err != nil {
		l.Debug("Distribution API failed, trying registry-specific API")
		// Fallback to registry-specific API (Harbor, etc.)
		tags, err = t.getTagsFromRegistryAPI(registry, repository)
		if err != nil {
			return ServiceStatus{}, fmt.Errorf("failed to get tags from both APIs: %v", err)
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
	
	resp, err := http.Get(endpoint)
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
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("endpoint returned status %d", resp.StatusCode)
	}

	// Try to parse as Harbor-style artifacts response
	var artifacts []OCIArtifact
	if err := json.NewDecoder(resp.Body).Decode(&artifacts); err == nil {
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
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err == nil {
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
