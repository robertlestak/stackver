package extractor

import (
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	// Common patterns for extracting versions
	imageTagPattern = regexp.MustCompile(`^([^:]+):(.+)$`)
	semverPattern   = regexp.MustCompile(`v?(\d+\.\d+\.\d+.*)`)
	gitHashPattern  = regexp.MustCompile(`^[a-f0-9]{7,40}$`)
)

// ExtractVersion extracts a version from various formats
func ExtractVersion(value string) string {
	l := log.WithFields(log.Fields{
		"value": value,
	})
	l.Debug("extracting version")

	if value == "" {
		return ""
	}

	// Try image tag format: nginx:1.21.0 -> 1.21.0
	if matches := imageTagPattern.FindStringSubmatch(value); len(matches) == 3 {
		version := matches[2]
		l.Debugf("extracted version from image tag: %s", version)
		return version
	}

	// Try semver format: v1.21.0 -> 1.21.0
	if matches := semverPattern.FindStringSubmatch(value); len(matches) >= 2 {
		version := matches[1]
		l.Debugf("extracted semver: %s", version)
		return version
	}

	// Check if it's a git hash
	if gitHashPattern.MatchString(value) {
		l.Debugf("detected git hash: %s", value)
		return value
	}

	// Return as-is if no pattern matches
	l.Debugf("no pattern matched, returning as-is: %s", value)
	return strings.TrimSpace(value)
}

// IsImageTag checks if a value looks like a container image tag
func IsImageTag(value string) bool {
	return imageTagPattern.MatchString(value)
}

// IsSemver checks if a value looks like semantic version
func IsSemver(value string) bool {
	return semverPattern.MatchString(value)
}

// IsGitHash checks if a value looks like a git commit hash
func IsGitHash(value string) bool {
	return gitHashPattern.MatchString(value)
}
