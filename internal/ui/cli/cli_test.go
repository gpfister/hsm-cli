package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
)

func TestCLI_CommandStructure(t *testing.T) {
	cmds := rootCmd.Commands()

	expectedCmds := []string{
		"devices",
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
	if !cmdMap["list"] {
		t.Errorf("expected alias 'list' for devices command")
	}
}

func TestCLI_PersistentFlags(t *testing.T) {
	portFlag := rootCmd.PersistentFlags().Lookup("port")
	if portFlag == nil {
		t.Fatalf("expected --port persistent flag to be registered")
	}
	if portFlag.Shorthand != "p" {
		t.Errorf("expected shorthand 'p' for --port, got %q", portFlag.Shorthand)
	}

	serialFlag := rootCmd.PersistentFlags().Lookup("serial")
	if serialFlag == nil {
		t.Fatalf("expected --serial persistent flag to be registered")
	}
	if serialFlag.Shorthand != "s" {
		t.Errorf("expected shorthand 's' for --serial, got %q", serialFlag.Shorthand)
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

func TestResetCmd_FlagRegistration(t *testing.T) {
	cmd := newResetCmd()
	yesFlag := cmd.Flags().Lookup("yes")
	if yesFlag == nil {
		t.Fatalf("expected --yes flag on reset command")
	}
	if yesFlag.Shorthand != "y" {
		t.Errorf("expected shorthand 'y' for --yes flag, got %q", yesFlag.Shorthand)
	}
}

func TestResetCmd_InteractiveCancellation(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"ExplicitNo", "no\n"},
		{"ShortN", "n\n"},
		{"EmptyInput", "\n"},
		{"ArbitraryWord", "abort\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newResetCmd()
			inBuf := bytes.NewBufferString(tc.input)
			outBuf := new(bytes.Buffer)
			cmd.SetIn(inBuf)
			cmd.SetOut(outBuf)

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("expected nil error on cancellation, got %v", err)
			}

			out := outBuf.String()
			if !strings.Contains(out, "WARNING: This operation will perform a destructive factory reset") {
				t.Errorf("expected warning banner in output, got: %s", out)
			}
			if !strings.Contains(out, "Are you sure you want to proceed? (yes/no):") {
				t.Errorf("expected confirmation prompt in output, got: %s", out)
			}
			if !strings.Contains(out, "Reset cancelled.") {
				t.Errorf("expected 'Reset cancelled.' in output, got: %s", out)
			}
		})
	}
}

func TestResetCmd_YesFlagBypassesPrompt(t *testing.T) {
	cmd := newResetCmd()
	outBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)
	cmd.SetArgs([]string{"--yes"})

	// Execute will attempt getATClient(), but should not prompt
	_ = cmd.Execute()

	out := outBuf.String()
	if strings.Contains(out, "Are you sure you want to proceed?") {
		t.Errorf("expected --yes flag to bypass interactive prompt, but prompt was displayed: %s", out)
	}
	if strings.Contains(out, "Reset cancelled.") {
		t.Errorf("expected --yes flag not to cancel, got: %s", out)
	}
}

func TestErrMultipleDevicesDetected_Format(t *testing.T) {
	err := &domain.ErrMultipleDevicesDetected{
		Devices: []domain.DiscoveredDongle{
			{
				Serial:   "F4CE368F12AB",
				Product:  "nRF52840 HSM Dongle",
				DataPort: "/dev/tty.usbmodemF4CE368F12AB3",
			},
			{
				Serial:   "FD54E793A75540C9",
				Product:  "nRF52840 HSM Dongle",
				DataPort: "/dev/tty.usbmodemFD54E793A75540C93",
			},
		},
	}

	msg := err.Error()
	if !strings.Contains(msg, "[1] Serial/BLE: F4CE368F12AB") {
		t.Errorf("expected device 1 formatted in error, got: %s", msg)
	}
	if !strings.Contains(msg, "[2] Serial/BLE: FD54E793A75540C9") {
		t.Errorf("expected device 2 formatted in error, got: %s", msg)
	}
	if !strings.Contains(msg, "--port (-p) or --serial (-s)") {
		t.Errorf("expected disambiguation advice in error, got: %s", msg)
	}
}
