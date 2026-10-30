package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLI_CommandStructure(t *testing.T) {
	cmds := rootCmd.Commands()

	expectedCmds := []string{
		"info",
		"status",
		"auth",
		"init",
		"lock",
		"reset",
		"key",
		"random",
		"pkcs11",
		"echo",
		"firmware",
		"test",
		"tui",
	}

	cmdMap := make(map[string]bool)
	for _, c := range cmds {
		cmdMap[c.Name()] = true
		for _, alias := range c.Aliases {
			cmdMap[alias] = true
		}
	}

	for _, name := range expectedCmds {
		if !cmdMap[name] {
			t.Errorf("expected root command %q to be registered", name)
		}
	}
	if !cmdMap["p11"] {
		t.Errorf("expected alias 'p11' for pkcs11 command group")
	}
}

func TestCLI_KeySubcommands(t *testing.T) {
	keyCmd := newKeyCmd()
	subCmds := keyCmd.Commands()

	expected := []string{"list", "generate", "pubkey", "sign", "migrate"}
	found := make(map[string]bool)
	for _, c := range subCmds {
		found[c.Name()] = true
	}

	for _, name := range expected {
		if !found[name] {
			t.Errorf("expected key subcommand %q to be registered", name)
		}
	}
}

func TestCLI_P11Subcommands(t *testing.T) {
	p11Cmd := newP11Cmd()
	subCmds := p11Cmd.Commands()

	expected := []string{"info", "slots", "init-token", "keygen", "sign", "test"}
	found := make(map[string]bool)
	for _, c := range subCmds {
		found[c.Name()] = true
	}

	for _, name := range expected {
		if !found[name] {
			t.Errorf("expected pkcs11 subcommand %q to be registered", name)
		}
	}
}

func TestCLI_HelpOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("help execution failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "hsmutil") {
		t.Errorf("expected help output to mention hsmutil, got: %s", output)
	}
}
