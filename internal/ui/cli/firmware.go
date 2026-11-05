package cli

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

type dfuProgressMsg struct {
	sent  int
	total int
	pct   float64
}

type dfuDoneMsg struct{}
type dfuErrMsg struct{ err error }

type dfuModel struct {
	progress progress.Model
	fileName string
	sent     int
	total    int
	pct      float64
	err      error
	done     bool
}

func newDFUModel(fileName string) dfuModel {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)
	return dfuModel{
		progress: p,
		fileName: fileName,
	}
}

func (m dfuModel) Init() tea.Cmd {
	return nil
}

func (m dfuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.err = errors.New("firmware update cancelled by user")
			return m, tea.Quit
		}
		return m, nil

	case dfuProgressMsg:
		m.sent = msg.sent
		m.total = msg.total
		m.pct = msg.pct
		cmd := m.progress.SetPercent(msg.pct / 100.0)
		return m, cmd

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case dfuDoneMsg:
		m.done = true
		return m, tea.Quit

	case dfuErrMsg:
		m.err = msg.err
		return m, tea.Quit

	default:
		return m, nil
	}
}

func (m dfuModel) View() string {
	if m.done {
		return fmt.Sprintf("Updating Firmware [%s]\n%s\n100%% complete (%d/%d bytes)\n",
			m.fileName, m.progress.View(), m.total, m.total)
	}
	if m.err != nil {
		return fmt.Sprintf("Firmware update failed: %v\n", m.err)
	}
	return fmt.Sprintf("Updating Firmware [%s]\n\n%s\n\nTransferring: %d / %d bytes (%.1f%%)\n",
		m.fileName, m.progress.View(), m.sent, m.total, m.pct)
}

func newFirmwareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firmware",
		Short: "Manage firmware (info, update)",
	}

	cmd.AddCommand(newFirmwareInfoCmd())
	cmd.AddCommand(newFirmwareUpdateCmd())
	return cmd
}

func newFirmwareInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Query active firmware version and MCUboot partition status",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := getService()
			if err != nil {
				return err
			}
			defer cleanup()

			info, err := svc.GetFirmwareInfo()
			if err != nil {
				return fmt.Errorf("failed to get firmware info: %w", err)
			}

			fmt.Println("==================================================")
			fmt.Println("                HSM Firmware Info                 ")
			fmt.Println("==================================================")
			fmt.Printf(" Firmware Version: %s\n", info.VersionString())
			fmt.Printf(" Active Partition: %s\n", info.SlotString())
			fmt.Printf(" Image Size:       %d bytes (0x%X)\n", info.ImageSize, info.ImageSize)
			if len(info.ImageSHA256) > 0 {
				fmt.Printf(" Image SHA-256:    %s\n", hex.EncodeToString(info.ImageSHA256))
			}
			fmt.Println("==================================================")
			return nil
		},
	}
}

func newFirmwareUpdateCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update firmware from a signed binary file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				return fmt.Errorf("--file flag is required")
			}

			svc, cleanup, err := getService()
			if err != nil {
				return err
			}
			defer cleanup()

			fmt.Printf("[INFO] Starting firmware streaming from: %s\n", filePath)

			isInteractive := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

			if isInteractive {
				m := newDFUModel(filepath.Base(filePath))
				p := tea.NewProgram(m)

				go func() {
					err := svc.UpdateFirmware(filePath, func(sent, total int, pct float64) {
						p.Send(dfuProgressMsg{sent: sent, total: total, pct: pct})
					})
					if err != nil {
						p.Send(dfuErrMsg{err: err})
					} else {
						p.Send(dfuDoneMsg{})
					}
				}()

				finalModel, err := p.Run()
				if err != nil {
					return fmt.Errorf("firmware update UI error: %w", err)
				}
				if fm, ok := finalModel.(dfuModel); ok && fm.err != nil {
					return fmt.Errorf("firmware update failed: %w", fm.err)
				}
			} else {
				// Non-interactive / headless fallback: linear periodic stdout logging
				var lastPct int = -1
				progressFn := func(sent, total int, pct float64) {
					currPct := int(pct)
					if currPct/10 > lastPct/10 || sent == total {
						lastPct = currPct
						fmt.Printf("[INFO] DFU Transfer: %3.0f%% complete (%d/%d bytes)\n", pct, sent, total)
					}
				}

				if err := svc.UpdateFirmware(filePath, progressFn); err != nil {
					return fmt.Errorf("firmware update failed: %w", err)
				}
			}

			fmt.Println("✓ [SUCCESS] Firmware update streamed and candidate slot validated.")
			fmt.Println("✓ [INFO] Device has been requested to swap into MCUboot.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to signed firmware binary (.bin)")
	_ = cmd.MarkFlagRequired("file")

	return cmd
}
