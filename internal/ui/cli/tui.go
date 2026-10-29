package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gpfister/nrf52840-hms-cli/internal/ui/tui"
	"github.com/spf13/cobra"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive terminal dashboard (BubbleTea)",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := getService()
			if err != nil {
				return err
			}
			defer cleanup()

			p := tea.NewProgram(tui.NewModel(svc))
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}
			return nil
		},
	}
}
