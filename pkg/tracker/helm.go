package tracker

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/robertlestak/stackver/pkg/utils"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type HelmTracker struct {
	uri              string
	acceptPrerelease bool
}

type HelmIndexEntry struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	Created     string `yaml:"created"`
}

type HelmIndex struct {
	Entries map[string][]HelmIndexEntry `yaml:"entries"`
}

func (t *HelmTracker) GetStatus(currentVersion string) (ServiceStatus, error) {
	return t.GetStatusWithOffset(currentVersion, 0)
}

func (t *HelmTracker) GetStatusWithOffset(currentVersion string, offset int) (ServiceStatus, error) {
	l := log.WithFields(log.Fields{
		"tracker": "helm",
		"uri":     t.uri,
	})
	l.Debug("getting status")

	// Parse URI format: "repo_url/chart_name"
	parts := strings.Split(t.uri, "/")
	if len(parts) < 2 {
		return ServiceStatus{}, fmt.Errorf("invalid helm URI format, expected 'repo_url/chart_name'")
	}
	
	chartName := parts[len(parts)-1]
	repoURL := strings.Join(parts[:len(parts)-1], "/")
	
	// Ensure repo URL has index.yaml
	if !strings.HasSuffix(repoURL, "/") {
		repoURL += "/"
	}
	indexURL := repoURL + "index.yaml"
	
	l.Debugf("getting endpoint: %s", indexURL)
	
	resp, err := http.Get(indexURL)
	if err != nil {
		return ServiceStatus{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ServiceStatus{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var index HelmIndex
	if err := yaml.NewDecoder(resp.Body).Decode(&index); err != nil {
		return ServiceStatus{}, err
	}

	entries, exists := index.Entries[chartName]
	if !exists {
		return ServiceStatus{}, fmt.Errorf("chart %s not found in repository", chartName)
	}

	if len(entries) == 0 {
		return ServiceStatus{}, fmt.Errorf("no versions found for chart %s", chartName)
	}

	// Extract all version strings
	var allVersions []string
	for _, entry := range entries {
		allVersions = append(allVersions, entry.Version)
	}
	
	// Get version at offset
	targetVersion := utils.GetVersionAtOffset(allVersions, offset, t.acceptPrerelease)
	if targetVersion == "" {
		return ServiceStatus{}, fmt.Errorf("no suitable version found for chart %s", chartName)
	}

	status := ServiceStatus{
		LatestVersion: targetVersion,
		Link:          repoURL,
	}
	
	status.CalculateStatus(currentVersion)
	
	l.Debugf("got status: %+v", status)
	return status, nil
}

func (t *HelmTracker) URI() string {
	return t.uri
}

func (t *HelmTracker) Link() string {
	parts := strings.Split(t.uri, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[:len(parts)-1], "/")
	}
	return t.uri
}
