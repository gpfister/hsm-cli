package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// Build metadata injected at link time via -ldflags "-X ...".
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Info holds resolved version and build metadata.
type Info struct {
	Version   string
	Commit    string
	BuildDate string
	GoVersion string
}

// String formats Info as:
// Version <version> (<short_commit_sha>) built on <build_timestamp_utc> (Go: <go_version>)
func (i Info) String() string {
	return fmt.Sprintf("Version %s (%s) built on %s (Go: %s)", i.Version, i.Commit, i.BuildDate, i.GoVersion)
}

// Resolve returns the resolved build metadata, falling back to runtime/debug.ReadBuildInfo()
// for Commit and BuildDate when not injected via -ldflags.
func Resolve() Info {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		bi = nil
	}
	return resolveWithBuildInfo(Version, Commit, BuildDate, runtime.Version(), bi)
}

// String returns the formatted version and build metadata string.
func String() string {
	return Resolve().String()
}

func resolveWithBuildInfo(ver, commit, buildDate, goVersion string, bi *debug.BuildInfo) Info {
	if strings.TrimSpace(ver) == "" {
		ver = "dev"
	}
	if strings.TrimSpace(commit) == "" {
		commit = "unknown"
	}
	if strings.TrimSpace(buildDate) == "" {
		buildDate = "unknown"
	}

	if bi != nil && (commit == "unknown" || buildDate == "unknown") {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if commit == "unknown" && s.Value != "" {
					if len(s.Value) >= 7 {
						commit = s.Value[:7]
					} else {
						commit = s.Value
					}
				}
			case "vcs.time":
				if buildDate == "unknown" && s.Value != "" {
					if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
						buildDate = t.UTC().Format("2006-01-02T15:04:05Z")
					}
				}
			}
		}
	}

	return Info{
		Version:   ver,
		Commit:    commit,
		BuildDate: buildDate,
		GoVersion: goVersion,
	}
}
