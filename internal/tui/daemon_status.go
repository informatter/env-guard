package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/env-guard/env-guard/internal/daemon"
)

type emptyConfigProvider struct{}

func (e *emptyConfigProvider) HasApp(_ string) bool           { return true }
func (e *emptyConfigProvider) AllowedPaths(_ string) []string { return nil }


// toggleDaemon starts or stops the daemon and updates the model state accordingly.
func toggleDaemon(m model) (tea.Model, tea.Cmd) {
	if m.daemon == nil {
		socketPath := m.daemonSocketPath
		if socketPath == "" {
			var err error
			socketPath, err = daemon.DefaultSocketPath()
			if err != nil {
				m.daemonError = fmt.Sprintf("socket path: %v", err)
				return m, nil
			}
		}
		cp := daemon.ConfigProvider(m.cfg)
		if cp == nil {
			cp = &emptyConfigProvider{}
		}
		m.daemon = daemon.New(m.vault, cp, socketPath)
	}

	if m.daemonRunning {
		if err := m.daemon.Stop(); err != nil {
			m.daemonError = fmt.Sprintf("stop: %v", err)
			return m, nil
		}
		m.daemonRunning = false
		m.daemonError = ""
	} else {
		if err := m.daemon.Start(); err != nil {
			m.daemonError = fmt.Sprintf("start: %v", err)
			m.daemonRunning = false
		} else {
			m.daemonRunning = true
			m.daemonError = ""
		}
	}

	return m, nil
}

// daemonStatusView returns a styled string representing the current status of the daemon.
func daemonStatusView(m model) string {
	if m.daemonError != "" {
		return daemonErrorStyle.Render("✗ " + m.daemonError)
	}
	if m.daemonRunning {
		return daemonRunningStyle.Render("● Daemon running — " + m.daemon.SocketPath())
	}
	return daemonStoppedStyle.Render("○ Daemon stopped")
}
