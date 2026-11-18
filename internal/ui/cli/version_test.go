package cli

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/gpfister/nrf52840-hsm-cli/internal/version"
)

func TestCLI_VersionCommand(t *testing.T) {
	origVersion, origCommit, origBuildDate := version.Version, version.Commit, version.BuildDate
	t.Cleanup(func() {
		version.Version, version.Commit, version.BuildDate = origVersion, origCommit, origBuildDate
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	version.Version = "v1.2.3"
	version.Commit = "1234567"
	version.BuildDate = "2026-11-18T08:30:00Z"

	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"version"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute() returned unexpected error: %v (stderr: %s)", err, stderr.String())
	}

	got := stdout.String()
	want := fmt.Sprintf("Version v1.2.3 (1234567) built on 2026-11-18T08:30:00Z (Go: %s)\n", runtime.Version())
	if got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}

	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected single-line output, got %d lines: %q", len(lines), got)
	}
}

func TestCLI_VersionCommandRejectsArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"version", "extra"})

	if err := rootCmd.Execute(); err == nil {
		t.Fatalf("expected error when passing extra arguments to 'version', got nil")
	}
}

func TestCLI_NoRootVersionFlag(t *testing.T) {
	if rootCmd.Version != "" {
		t.Fatalf("expected rootCmd.Version to be empty, got %q", rootCmd.Version)
	}
	if f := rootCmd.Flags().Lookup("version"); f != nil {
		t.Fatalf("expected --version flag not to be registered on rootCmd")
	}
	if f := rootCmd.PersistentFlags().Lookup("version"); f != nil {
		t.Fatalf("expected --version persistent flag not to be registered on rootCmd")
	}
	if f := rootCmd.Flags().ShorthandLookup("v"); f != nil {
		t.Fatalf("expected -v shorthand flag not to be registered on rootCmd")
	}
	if f := rootCmd.PersistentFlags().ShorthandLookup("v"); f != nil {
		t.Fatalf("expected -v persistent shorthand flag not to be registered on rootCmd")
	}
}
