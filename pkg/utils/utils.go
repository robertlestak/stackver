package utils

import (
	"sort"
	"strings"

	"gopkg.in/Masterminds/semver.v1"
)

func CycleContainsVersion(cycle string, version string) bool {
	if cycle == "" || version == "" {
		return false
	}
	if cycle == version {
		return true
	}
	if strings.Contains(cycle, version) {
		return true
	}
	cycleVersion, err := semver.NewVersion(cycle)
	if err != nil {
		// Handle the error, for example, return false if cycle is not a valid SemVer
		return false
	}
	versionToCompare, err := semver.NewVersion(version)
	if err != nil {
		// Handle the error, for example, return false if version is not a valid SemVer
		return false
	}
	// ensure that version is in the major version of the cycle
	if cycleVersion.Major() != versionToCompare.Major() {
		return false
	}
	// ensure that the minor version of the cycle is less than or equal to the minor version of the version
	if cycleVersion.Minor() >= versionToCompare.Minor() {
		return true
	}
	// ensure that the patch version of the cycle is less than or equal to the patch version of the version
	if cycleVersion.Patch() >= versionToCompare.Patch() {
		return true
	}
	return false
}

func TrimVersionPrefix(version string) string {
	// Only trim "v" if it's followed by a digit (actual version prefix)
	// This prevents removing "v" from words like "vertical"
	if len(version) > 1 && version[0] == 'v' && version[1] >= '0' && version[1] <= '9' {
		return strings.TrimPrefix(version, "v")
	}
	return version
}

func IsPrerelease(version string) bool {
	// Check for common prerelease indicators
	lowerVersion := strings.ToLower(version)
	prereleaseKeywords := []string{"-rc", "-alpha", "-beta", "-dev", "-snapshot", "-pre", ".rc", ".alpha", ".beta"}
	
	for _, keyword := range prereleaseKeywords {
		if strings.Contains(lowerVersion, keyword) {
			return true
		}
	}
	
	return false
}

func IsDowngrade(currentVersion, latestVersion string) bool {
	if currentVersion == "" || latestVersion == "" {
		return false
	}
	
	// Clean versions for comparison
	current := TrimVersionPrefix(currentVersion)
	latest := TrimVersionPrefix(latestVersion)
	
	currentSemver, err := semver.NewVersion(current)
	if err != nil {
		return false
	}
	
	latestSemver, err := semver.NewVersion(latest)
	if err != nil {
		return false
	}
	
	// Return true if current version is greater than "latest" version
	return currentSemver.GreaterThan(latestSemver)
}

func GetVersionAtOffset(versions []string, offset int, acceptPrerelease bool) string {
	if len(versions) == 0 {
		return ""
	}
	
	// Filter and sort versions
	var validVersions []*semver.Version
	versionMap := make(map[string]string)
	
	for _, v := range versions {
		cleanVersion := TrimVersionPrefix(v)
		if semVer, err := semver.NewVersion(cleanVersion); err == nil {
			// Skip prereleases if not accepted
			if !acceptPrerelease && IsPrerelease(v) {
				continue
			}
			validVersions = append(validVersions, semVer)
			versionMap[semVer.String()] = v
		}
	}
	
	if len(validVersions) == 0 {
		return ""
	}
	
	// Sort newest first
	sort.Sort(sort.Reverse(semver.Collection(validVersions)))
	
	// Apply offset (0 = latest, 1 = N-1, 2 = N-2, etc.)
	if offset >= len(validVersions) {
		offset = len(validVersions) - 1
	}
	
	targetVersion := validVersions[offset]
	return versionMap[targetVersion.String()]
}
