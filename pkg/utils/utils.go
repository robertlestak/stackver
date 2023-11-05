package utils

import (
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
	// version is in format "v1.2.3"
	// trim the "v" prefix
	return strings.TrimPrefix(version, "v")
}
