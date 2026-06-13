package tui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/env-guard/env-guard/internal/config"
	"github.com/env-guard/env-guard/internal/daemon"
	"github.com/env-guard/env-guard/internal/vault"
)

type screen int

const (
	screenSetup screen = iota
	screenPassword
	screenDashboard
	screenAccessLog
	screenRecovery
)

type secretItem struct {
	key   string
	value string
	set   bool
}

type errMsg struct {
	err error
}

type model struct {
	screen    screen
	vaultPath string
	vault     *vault.SQLiteVault
	cfg       *config.Config
	err       error
	width     int
	height    int
	quitting  bool

	setupStep     int
	setupPassword string
	setupConfirm  string
	setupError    string
	setupInput    textinput.Model
	hasConfig     bool
	creating      bool

	password      string
	passwordError string
	passwordInput textinput.Model

	projects        []string
	selectedProject int
	selectedSecret  int
	secrets         []secretItem
	focusPanel      int
	editingSecret   int
	showValues      bool
	savedMessage    string
	editInput       textinput.Model

	daemonSocketPath string
	daemon           *daemon.Daemon
	daemonRunning    bool
	daemonError      string
	accessLog        []vault.AccessEntry
	accessLogScroll  int

	recoveryStep   int
	recoveryRootPw string
	recoveryNewPw  string
	recoveryError  string
	recoveryInput  textinput.Model
}

func Run() int {
	vaultPath, err := vault.DefaultVaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	cfgPath := findConfig()
	var cfg *config.Config
	hasConfig := false
	if cfgPath != "" {
		cfg, err = config.Parse(cfgPath)
		if err == nil {
			hasConfig = true
		}
	}

	vaultExists := false
	if _, err := os.Stat(vaultPath); err == nil {
		vaultExists = true
	}

	ti := textinput.New()
	ti.Prompt = "▸ "
	ti.EchoMode = textinput.EchoPassword
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 40

	socketPath, _ := daemon.DefaultSocketPath()

	m := model{
		vaultPath:        vaultPath,
		vault:            vault.New(vaultPath),
		cfg:              cfg,
		hasConfig:        hasConfig,
		daemonSocketPath: socketPath,
	}

	if vaultExists {
		m.screen = screenPassword
		m.passwordInput = ti
	} else {
		m.screen = screenSetup
		m.setupInput = ti
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return 1
	}
	return 0
}

func findConfig() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	path := filepath.Join(cwd, "env-guard.yaml")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.quitting {
			switch msg.String() {
			case "y", "Y":
				if m.daemon != nil && m.daemonRunning {
					m.daemon.Stop()
				}
				return m, tea.Quit
			default:
				m.quitting = false
				return m, nil
			}
		}

		switch msg.String() {
		case "ctrl+c":
			if m.screen == screenDashboard {
				m.quitting = true
				return m, nil
			}
			return m, tea.Quit
		}

		switch m.screen {
		case screenSetup:
			return updateSetup(msg, m)
		case screenPassword:
			return updatePassword(msg, m)
		case screenDashboard:
			return updateDashboard(msg, m)
		case screenAccessLog:
			return updateAccessLog(msg, m)
		case screenRecovery:
			return updateRecovery(msg, m)
		}

	case vaultCreatedMsg:
		return loadDashboard(m)
	case vaultOpenedMsg:
		return loadDashboard(m)
	case errMsg:
		m.err = msg.err
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return quitView(m)
	}
	switch m.screen {
	case screenSetup:
		return setupView(m)
	case screenPassword:
		return passwordView(m)
	case screenDashboard:
		return dashboardView(m)
	case screenAccessLog:
		return accessLogView(m)
	case screenRecovery:
		return recoveryView(m)
	}
	return ""
}

func quitView(m model) string {
	s := "\n  Are you sure you want to quit?\n\n"
	s += "    > y/yes   Quit\n"
	s += "    > any key Cancel\n\n"
	return s
}
