package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"testing"
)

func TestString_InjectedMetadata(t *testing.T) {
	origVersion, origCommit, origBuildDate := Version, Commit, BuildDate
	t.Cleanup(func() {
		Version, Commit, BuildDate = origVersion, origCommit, origBuildDate
	})

	Version = "v1.2.0"
	Commit = "a1b2c3d"
	BuildDate = "2026-11-17T17:30:00Z"

	got := String()
	want := fmt.Sprintf("Version v1.2.0 (a1b2c3d) built on 2026-11-17T17:30:00Z (Go: %s)", runtime.Version())
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestResolveWithBuildInfo_FallbackDefaults(t *testing.T) {
	info := resolveWithBuildInfo("dev", "unknown", "unknown", "go1.27.4", nil)
	want := "Version dev (unknown) built on unknown (Go: go1.27.4)"
	if got := info.String(); got != want {
		t.Fatalf("info.String() = %q, want %q", got, want)
	}
}

func TestResolveWithBuildInfo_VCSFallback(t *testing.T) {
	bi := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "0123456789abcdef0123456789abcdef01234567"},
			{Key: "vcs.time", Value: "2026-11-18T09:15:30+02:00"},
		},
	}

	info := resolveWithBuildInfo("dev", "unknown", "unknown", "go1.27.4", bi)
	want := "Version dev (0123456) built on 2026-11-18T07:15:30Z (Go: go1.27.4)"
	if got := info.String(); got != want {
		t.Fatalf("info.String() = %q, want %q", got, want)
	}
}

func TestResolveWithBuildInfo_ShortAndMalformedVCS(t *testing.T) {
	bi := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc12"},
			{Key: "vcs.time", Value: "not-a-timestamp"},
		},
	}

	info := resolveWithBuildInfo("", "", "", "go1.27.4", bi)
	want := "Version dev (abc12) built on unknown (Go: go1.27.4)"
	if got := info.String(); got != want {
		t.Fatalf("info.String() = %q, want %q", got, want)
	}
}

func TestResolveWithBuildInfo_InjectedOverridesBuildInfo(t *testing.T) {
	bi := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "9999999999999"},
			{Key: "vcs.time", Value: "2020-01-01T00:00:00Z"},
		},
	}

	info := resolveWithBuildInfo("v2.0.0", "deadbee", "2026-11-18T08:00:00Z", "go1.27.4", bi)
	want := "Version v2.0.0 (deadbee) built on 2026-11-18T08:00:00Z (Go: go1.27.4)"
	if got := info.String(); got != want {
		t.Fatalf("info.String() = %q, want %q", got, want)
	}
}
