package tracker

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/robertlestak/stackver/pkg/utils"
	log "github.com/sirupsen/logrus"
)

type GitTracker struct {
	uri              string
	acceptPrerelease bool
}

func (t *GitTracker) GetStatus(currentVersion string) (ServiceStatus, error) {
	return t.GetStatusWithOffset(currentVersion, 0)
}

func (t *GitTracker) GetStatusWithOffset(currentVersion string, offset int) (ServiceStatus, error) {
	l := log.WithFields(log.Fields{
		"tracker": "git",
		"uri":     t.uri,
	})
	l.Debug("getting status")

	// Use git ls-remote to get tags without cloning
	cmd := exec.Command("git", "ls-remote", "--tags", t.uri)
	output, err := cmd.Output()
	if err != nil {
		return ServiceStatus{}, fmt.Errorf("failed to fetch git tags: %w", err)
	}

	// Parse git ls-remote output
	var tags []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		// Format: <commit-hash>\trefs/tags/<tag-name>
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			continue
		}
		
		ref := parts[1]
		if !strings.HasPrefix(ref, "refs/tags/") {
			continue
		}
		
		// Skip annotated tag references (^{})
		if strings.HasSuffix(ref, "^{}") {
			continue
		}
		
		tag := strings.TrimPrefix(ref, "refs/tags/")
		tags = append(tags, tag)
	}

	if len(tags) == 0 {
		return ServiceStatus{}, fmt.Errorf("no tags found in repository")
	}

	// Get version at offset
	targetVersion := utils.GetVersionAtOffset(tags, offset, t.acceptPrerelease)
	if targetVersion == "" {
		return ServiceStatus{}, fmt.Errorf("no suitable version found")
	}

	status := ServiceStatus{
		LatestVersion: utils.TrimVersionPrefix(targetVersion),
		Link:          t.uri,
	}
	
	status.CalculateStatus(currentVersion)
	
	l.Debugf("got status: %+v", status)
	return status, nil
}

func (t *GitTracker) URI() string {
	return t.uri
}

func (t *GitTracker) Link() string {
	return t.uri
}
