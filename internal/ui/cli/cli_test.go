package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
)

func TestCLI_CommandStructure(t *testing.T) {
	cmds := rootCmd.Commands()

	// Visible top-level commands
	expectedVisible := []string{
		"device",
		"devices",
		"unlock",
		"key",
		"random",
		"pkcs11",
		"echo",
	}

	cmdMap := make(map[string]bool)
	hiddenMap := make(map[string]bool)
	for _, c := range cmds {
		cmdMap[c.Name()] = true
		if c.Hidden {
			hiddenMap[c.Name()] = true
		}
		for _, alias := range c.Aliases {
			cmdMap[alias] = true
			if c.Hidden {
				hiddenMap[alias] = true
			}
		}
	}

	for _, name := range expectedVisible {
		if !cmdMap[name] {
			t.Errorf("expected root command %q to be registered", name)
		}
		if hiddenMap[name] {
			t.Errorf("expected root command %q to be visible, but it is marked hidden", name)
		}
	}

	if !cmdMap["p11"] {
		t.Errorf("expected alias 'p11' for pkcs11 command group")
	}
	if !cmdMap["auth"] {
		t.Errorf("expected alias 'auth' for unlock command")
	}

	// Legacy aliases must be registered and hidden
	legacyHidden := []string{
		"info",
		"status",
		"init",
		"lock",
		"reset",
		"firmware",
	}
	for _, name := range legacyHidden {
		if !cmdMap[name] {
			t.Errorf("expected legacy command %q to be registered on rootCmd for backward compatibility", name)
		}
		if !hiddenMap[name] {
			t.Errorf("expected legacy command %q to be hidden on rootCmd, but it is visible", name)
		}
	}

	// Pruned commands must NOT be registered
	prunedCmds := []string{"test", "tui"}
	for _, name := range prunedCmds {
		if cmdMap[name] {
			t.Errorf("expected pruned command %q to NOT be registered on rootCmd", name)
		}
	}
}

func TestCLI_DeviceSubcommands(t *testing.T) {
	devCmd := newDeviceCmd()
	subCmds := devCmd.Commands()

	expected := []string{"list", "info", "init", "lock", "reset", "status", "migrate", "firmware"}
	found := make(map[string]bool)
	aliasFound := make(map[string]bool)
	for _, c := range subCmds {
		found[c.Name()] = true
		for _, a := range c.Aliases {
			aliasFound[a] = true
		}
	}

	for _, name := range expected {
		if !found[name] {
			t.Errorf("expected device subcommand %q to be registered", name)
		}
	}

	if !aliasFound["devices"] || !aliasFound["ls"] {
		t.Errorf("expected aliases 'devices' and 'ls' on device list command")
	}
}

func TestCLI_FirmwareSubcommands(t *testing.T) {
	fwCmd := newFirmwareCmd()
	subCmds := fwCmd.Commands()

	expected := []string{"info", "update"}
	found := make(map[string]bool)
	for _, c := range subCmds {
		found[c.Name()] = true
	}

	for _, name := range expected {
		if !found[name] {
			t.Errorf("expected firmware subcommand %q to be registered", name)
		}
	}
}

func TestCLI_DeviceMigrateCmd_Flags(t *testing.T) {
	cmd := newDeviceMigrateCmd()
	targetFlag := cmd.Flags().Lookup("target")
	if targetFlag == nil {
		t.Fatalf("expected --target flag on device migrate")
	}
	if targetFlag.Shorthand != "t" {
		t.Errorf("expected shorthand 't' for --target, got %q", targetFlag.Shorthand)
	}

	sourceFlag := cmd.Flags().Lookup("source")
	if sourceFlag == nil {
		t.Fatalf("expected --source flag on device migrate")
	}
	if sourceFlag.Shorthand != "s" {
		t.Errorf("expected shorthand 's' for --source, got %q", sourceFlag.Shorthand)
	}

	zeroizeFlag := cmd.Flags().Lookup("zeroize-source")
	if zeroizeFlag == nil {
		t.Fatalf("expected --zeroize-source flag on device migrate")
	}

	yesFlag := cmd.Flags().Lookup("yes")
	if yesFlag == nil {
		t.Fatalf("expected --yes flag on device migrate")
	}
	if yesFlag.Shorthand != "y" {
		t.Errorf("expected shorthand 'y' for --yes, got %q", yesFlag.Shorthand)
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

	// Terminology neutrality check: no occurrences of nrf52840
	if strings.Contains(strings.ToLower(output), "nrf52840") {
		t.Errorf("expected terminology neutrality in help output, but found 'nrf52840': %s", output)
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
				Product:  "HSM USB Key",
				DataPort: "/dev/tty.usbmodemF4CE368F12AB3",
			},
			{
				Serial:   "FD54E793A75540C9",
				Product:  "HSM USB Key",
				DataPort: "/dev/tty.usbmodemFD54E793A75540C93",
			},
		},
	}

	msg := err.Error()
	if !strings.Contains(msg, "multiple HSM USB devices detected:") {
		t.Errorf("expected neutral header in error, got: %s", msg)
	}
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
