package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gpfister/nrf52840-hms-cli/internal/domain"
	"github.com/gpfister/nrf52840-hms-cli/internal/usecases"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1, 2)

	blueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00A0FF")).Bold(true)
	redStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3333")).Bold(true)
	greenStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF66")).Bold(true)
)

type tickMsg time.Time

type Model struct {
	svc          *usecases.HSMService
	devInfo      *domain.DeviceInfo
	fwInfo       *domain.FirmwareInfo
	statusText   string
	currentState domain.DeviceState
	lastLatency  time.Duration
	err          error
}

func NewModel(svc *usecases.HSMService) Model {
	return Model{
		svc:          svc,
		statusText:   "Ready. Press 'e' (echo), 'u' (unlock), 'l' (lock), 'r' (reset), 'q' (quit)",
		currentState: domain.StateUnconfigured,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.refreshCmd(),
		tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}

func (m Model) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		info, _ := m.svc.GetDeviceInfo()
		fw, _ := m.svc.GetFirmwareInfo()
		return refreshMsg{devInfo: info, fwInfo: fw}
	}
}

type refreshMsg struct {
	devInfo *domain.DeviceInfo
	fwInfo  *domain.FirmwareInfo
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "e":
			reply, lat, err := m.svc.Echo("TUI Ping")
			if err != nil {
				m.err = err
				m.statusText = fmt.Sprintf("Echo error: %v", err)
			} else {
				m.lastLatency = lat
				m.statusText = fmt.Sprintf("Echo reply: %s (latency %v)", reply, lat)
			}
			return m, nil

		case "u":
			if err := m.svc.Unlock(); err != nil {
				m.err = err
				m.statusText = fmt.Sprintf("Unlock error: %v", err)
			} else {
				m.currentState = domain.StateUnlocked
				m.statusText = "Device Unlocked! LED transitioned to solid GREEN."
			}
			return m, nil

		case "l":
			if err := m.svc.Lock(); err != nil {
				m.err = err
				m.statusText = fmt.Sprintf("Lock error: %v", err)
			} else {
				m.currentState = domain.StateLocked
				m.statusText = "Device Locked! LED transitioned to solid RED."
			}
			return m, nil

		case "r":
			if err := m.svc.Reset(); err != nil {
				m.err = err
				m.statusText = fmt.Sprintf("Reset error: %v", err)
			} else {
				m.currentState = domain.StateUnconfigured
				m.statusText = "Device Reset! LED transitioned to pulsating BLUE."
			}
			return m, nil
		}

	case refreshMsg:
		if msg.devInfo != nil {
			m.devInfo = msg.devInfo
		}
		if msg.fwInfo != nil {
			m.fwInfo = msg.fwInfo
		}
		return m, nil

	case tickMsg:
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	}

	return m, nil
}

func (m Model) View() string {
	s := titleStyle.Render("nRF52840 HSM Dongle Dashboard") + "\n\n"

	ledVisual := ""
	switch m.currentState {
	case domain.StateUnconfigured:
		ledVisual = blueStyle.Render("● PULSATING BLUE (Not Configured)")
	case domain.StateLocked:
		ledVisual = redStyle.Render("● SOLID RED (Configured & Locked)")
	case domain.StateUnlocked:
		ledVisual = greenStyle.Render("● SOLID GREEN (Authenticated & Unlocked)")
	}

	devStr := "Connecting..."
	if m.devInfo != nil {
		devStr = fmt.Sprintf("Serial: 0x%08X  |  Version: %s", m.devInfo.Serial, m.devInfo.VersionString())
	}

	fwStr := "Querying..."
	if m.fwInfo != nil {
		fwStr = fmt.Sprintf("Version: %s  |  %s", m.fwInfo.VersionString(), m.fwInfo.SlotString())
	}

	content := fmt.Sprintf(
		"Device Info:    %s\n"+
			"Firmware DFU:   %s\n"+
			"LED Indicator:  %s\n\n"+
			"Status:         %s\n\n"+
			"[e] Echo Check  [u] Unlock (Green)  [l] Lock (Red)  [r] Reset (Blue)  [q] Quit",
		devStr, fwStr, ledVisual, m.statusText,
	)

	return boxStyle.Render(s + content)
}
