package tracker

import (
	"testing"

	"github.com/robertlestak/stackver/pkg/utils"
)

func TestVersionFromTagNameFiltersConfiguredPrefix(t *testing.T) {
	releases := []string{
		"v0.22.0",
		"external-dns-helm-chart-1.21.1",
		"v0.21.0",
		"external-dns-helm-chart-1.20.0",
	}

	var versions []string
	for _, release := range releases {
		version, ok := versionFromTagName(release, "external-dns-helm-chart-")
		if ok {
			versions = append(versions, version)
		}
	}

	latest := utils.GetVersionAtOffset(versions, 0, false)
	if latest != "1.21.1" {
		t.Fatalf("latest = %q, want 1.21.1", latest)
	}
}

func TestVersionFromTagNameAllowsUnprefixedTagsWhenPrefixUnset(t *testing.T) {
	version, ok := versionFromTagName("v1.2.3", "")
	if !ok {
		t.Fatal("expected unprefixed tag to be accepted")
	}
	if version != "v1.2.3" {
		t.Fatalf("version = %q, want v1.2.3", version)
	}
}
