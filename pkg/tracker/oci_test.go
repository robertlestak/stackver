package tracker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOCITrackerDistributionAPIBearerChallenge(t *testing.T) {
	var tokenRequested bool
	var authorizedTagsRequested bool

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/project/image/tags/list":
			if r.Header.Get("Authorization") != "Bearer test-token" {
				w.Header().Set("WWW-Authenticate", `Bearer realm="`+serverURL(r)+`/token",service="test-registry",scope="repository:project/image:pull"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			authorizedTagsRequested = true
			json.NewEncoder(w).Encode(OCITagsResponse{Tags: []string{"1.0.0", "1.2.0"}})
		case "/token":
			tokenRequested = true
			if got := r.URL.Query().Get("service"); got != "test-registry" {
				t.Fatalf("service query = %q, want test-registry", got)
			}
			if got := r.URL.Query().Get("scope"); got != "repository:project/image:pull" {
				t.Fatalf("scope query = %q, want repository:project/image:pull", got)
			}
			json.NewEncoder(w).Encode(OCITokenResponse{Token: "test-token"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	restoreDefaultClient := setDefaultClient(server.Client())
	defer restoreDefaultClient()

	tracker := &OCITracker{}
	registry := strings.TrimPrefix(server.URL, "https://")
	tags, err := tracker.getTagsFromDistributionAPI(registry, "project/image")
	if err != nil {
		t.Fatalf("getTagsFromDistributionAPI returned error: %v", err)
	}
	if !tokenRequested {
		t.Fatal("token endpoint was not requested")
	}
	if !authorizedTagsRequested {
		t.Fatal("authorized tags request was not made")
	}
	if got, want := strings.Join(tags, ","), "1.0.0,1.2.0"; got != want {
		t.Fatalf("tags = %q, want %q", got, want)
	}
}

func TestOCITrackerDistributionAPIWithoutAuth(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/project/image/tags/list" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(OCITagsResponse{Tags: []string{"1.0.0"}})
	}))
	defer server.Close()
	restoreDefaultClient := setDefaultClient(server.Client())
	defer restoreDefaultClient()

	tracker := &OCITracker{}
	registry := strings.TrimPrefix(server.URL, "https://")
	tags, err := tracker.getTagsFromDistributionAPI(registry, "project/image")
	if err != nil {
		t.Fatalf("getTagsFromDistributionAPI returned error: %v", err)
	}
	if got, want := strings.Join(tags, ","), "1.0.0"; got != want {
		t.Fatalf("tags = %q, want %q", got, want)
	}
}

func TestOCITrackerDistributionAPIRetriesTooManyRequests(t *testing.T) {
	var requests int

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		json.NewEncoder(w).Encode(OCITagsResponse{Tags: []string{"1.0.0"}})
	}))
	defer server.Close()
	restoreDefaultClient := setDefaultClient(server.Client())
	defer restoreDefaultClient()

	start := time.Now()
	tracker := &OCITracker{}
	registry := strings.TrimPrefix(server.URL, "https://")
	tags, err := tracker.getTagsFromDistributionAPI(registry, "project/image")
	if err != nil {
		t.Fatalf("getTagsFromDistributionAPI returned error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if time.Since(start) < time.Second {
		t.Fatal("request was not delayed by Retry-After")
	}
	if got, want := strings.Join(tags, ","), "1.0.0"; got != want {
		t.Fatalf("tags = %q, want %q", got, want)
	}
}

func TestParseWWWAuthenticate(t *testing.T) {
	scheme, params, err := parseWWWAuthenticate(`Bearer realm="https://public.ecr.aws/token/",service="public.ecr.aws",scope="aws"`)
	if err != nil {
		t.Fatalf("parseWWWAuthenticate returned error: %v", err)
	}
	if scheme != "Bearer" {
		t.Fatalf("scheme = %q, want Bearer", scheme)
	}
	if params["realm"] != "https://public.ecr.aws/token/" {
		t.Fatalf("realm = %q", params["realm"])
	}
	if params["service"] != "public.ecr.aws" {
		t.Fatalf("service = %q", params["service"])
	}
	if params["scope"] != "aws" {
		t.Fatalf("scope = %q", params["scope"])
	}
}

func serverURL(r *http.Request) string {
	return "https://" + r.Host
}

func setDefaultClient(client *http.Client) func() {
	originalClient := http.DefaultClient
	http.DefaultClient = client
	return func() {
		http.DefaultClient = originalClient
	}
}
