package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func openAccessLog(m model) (tea.Model, tea.Cmd) {
	entries, err := m.vault.AccessLog()
	if err != nil {
		m.err = err
		return m, nil
	}
	m.screen = screenAccessLog
	m.accessLog = entries
	m.accessLogScroll = 0
	return m, nil
}

func updateAccessLog(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.accessLogScroll > 0 {
			m.accessLogScroll--
		}
	case "down", "j":
		if m.accessLogScroll < len(m.accessLog)-1 {
			m.accessLogScroll++
		}
	case "esc", "q":
		m.screen = screenDashboard
	}
	return m, nil
}

func accessLogView(m model) string {
	s := logoStyle.Render("🔐 env-guard") + "\n\n"
	s += headerStyle.Render("Access Logs") + "\n\n"

	if len(m.accessLog) == 0 {
		s += dimmedStyle.Render("  No access log entries") + "\n"
		s += "\n" + helpStyle.Render("Press Esc to go back")
		return appStyle.Render(s)
	}

	header := accessLogHeaderStyle.Render(
		fmt.Sprintf("%-19s  %-12s  %-22s  %s", "Time", "App", "Process", "Keys"),
	)
	s += header + "\n"
	s += dimmedStyle.Render(strings.Repeat("─", 70)) + "\n"

	maxVisible := max(m.height-10, 5)

	start := m.accessLogScroll
	end := min(start+maxVisible, len(m.accessLog))

	for i, entry := range m.accessLog[start:end] {
		ts := time.Unix(entry.Timestamp, 0).Format("2006-01-02 15:04:05")
		app := entry.AppName
		proc := entry.ProcessPath
		keys := entry.KeysRequested

		if len(proc) > 20 {
			proc = "..." + proc[len(proc)-17:]
		}
		if len(keys) > 8 {
			keys = keys[:8] + "…"
		}

		cursor := "  "
		style := accessLogEntryStyle
		if i == 0 {
			cursor = "▸ "
			style = activeProjectStyle
		}

		line := fmt.Sprintf("%-19s  %-12s  %-22s  %s", ts, app, proc, keys)
		s += style.Render(cursor+line) + "\n"
	}

	if len(m.accessLog) > maxVisible {
		s += "\n" + helpStyle.Render(fmt.Sprintf("Showing %d–%d of %d entries", start+1, end, len(m.accessLog)))
	}

	s += "\n\n" + helpStyle.Render("↑↓: scroll  •  Esc: back")
	return appStyle.Render(s)
}
