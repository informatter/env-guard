package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/env-guard/env-guard/internal/vault"
)

type projectsLoadedMsg struct {
	projects []string
}

type secretsLoadedMsg struct {
	secrets []secretItem
}

type savedDoneMsg struct{}

func loadDashboard(m model) (tea.Model, tea.Cmd) {
	m.screen = screenDashboard

	projects, err := m.vault.Projects()
	if err != nil {
		m.err = err
		return m, nil
	}
	m.projects = projects
	m.selectedProject = 0
	m.selectedSecret = 0
	m.focusPanel = 0
	m.editingSecret = -1
	m.showValues = false
	m.savedMessage = ""

	if len(projects) > 0 {
		return loadSecrets(m)
	}

	m.secrets = nil
	return m, nil
}

func loadSecrets(m model) (tea.Model, tea.Cmd) {
	if m.selectedProject >= len(m.projects) {
		return m, nil
	}
	project := m.projects[m.selectedProject]

	keys, err := m.vault.SecretKeys(project)
	if err != nil {
		m.err = err
		return m, nil
	}

	secrets := make([]secretItem, len(keys))
	for i, key := range keys {
		value, err := m.vault.GetSecret(project, key)
		if err == vault.ErrSecretNotSet {
			secrets[i] = secretItem{key: key, value: "", set: false}
		} else if err != nil {
			secrets[i] = secretItem{key: key, value: "", set: false}
		} else {
			secrets[i] = secretItem{key: key, value: value, set: true}
		}
	}
	m.secrets = secrets
	m.selectedSecret = 0
	m.editingSecret = -1

	return m, nil
}

func updateDashboard(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	if m.editingSecret >= 0 {
		return updateEditSecret(msg, m)
	}

	switch msg.String() {
	case "tab":
		m.focusPanel = (m.focusPanel + 1) % 2
		return m, nil

	case "up", "k":
		if m.focusPanel == 0 && len(m.projects) > 0 {
			m.selectedProject = max(0, m.selectedProject-1)
			return loadSecrets(m)
		}
		if m.focusPanel == 1 && len(m.secrets) > 0 {
			m.selectedSecret = max(0, m.selectedSecret-1)
		}
		return m, nil

	case "down", "j":
		if m.focusPanel == 0 && len(m.projects) > 0 {
			m.selectedProject = min(len(m.projects)-1, m.selectedProject+1)
			return loadSecrets(m)
		}
		if m.focusPanel == 1 && len(m.secrets) > 0 {
			m.selectedSecret = min(len(m.secrets)-1, m.selectedSecret+1)
		}
		return m, nil

	case "enter":
		if m.focusPanel == 1 && len(m.secrets) > 0 {
			m.editingSecret = m.selectedSecret
			return startEditSecret(m)
		}
		return m, nil

	case "v":
		m.showValues = !m.showValues
		return m, nil

	case "d":
		return toggleDaemon(m)

	case "l":
		return openAccessLog(m)

	case "q":
		m.quitting = true
		return m, nil
	}

	if m.savedMessage != "" {
		m.savedMessage = ""
	}

	return m, nil
}

func updateEditSecret(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.editingSecret = -1
		m.editInput.Blur()
		return m, nil
	}

	if msg.String() == "ctrl+s" {
		return saveSecret(m)
	}

	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

func startEditSecret(m model) (tea.Model, tea.Cmd) {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Focus()
	ti.CharLimit = 4096
	ti.Width = max(m.width-10, 20)

	if m.selectedSecret >= 0 && m.selectedSecret < len(m.secrets) {
		secret := m.secrets[m.selectedSecret]
		if secret.set {
			ti.SetValue(secret.value)
		}
	}

	m.editInput = ti
	return m, nil
}

func saveSecret(m model) (tea.Model, tea.Cmd) {
	if m.editingSecret < 0 || m.editingSecret >= len(m.secrets) {
		m.editingSecret = -1
		return m, nil
	}

	project := m.projects[m.selectedProject]
	secret := m.secrets[m.editingSecret]
	value := m.editInput.Value()

	if err := m.vault.SetSecret(project, secret.key, value); err != nil {
		m.err = err
		m.editingSecret = -1
		return m, nil
	}

	m.editingSecret = -1
	m.savedMessage = "✓ Saved!"

	return loadSecrets(m)
}

func dashboardView(m model) string {
	s := logoStyle.Render("🔐 env-guard") + "\n\n"

	if m.savedMessage != "" {
		s += successStyle.Render(m.savedMessage) + "\n\n"
	}

	if len(m.projects) == 0 {
		s += headerStyle.Render("No applications configured") + "\n\n"
		s += "Create an " + labelStyle.Render("env-guard.yaml") + " file to define your apps.\n\n"
		s += helpStyle.Render("q: quit") + "\n"
		return appStyle.Render(s)
	}

	leftPanel := renderProjectList(m)
	rightPanel := renderSecretList(m)

	panels := []string{leftPanel, rightPanel}
	view := lipgloss.JoinHorizontal(lipgloss.Top, panels...)

	s += view + "\n\n"
	s += helpStyle.Render("Tab: switch panel  •  ↑↓: navigate  •  Enter: edit  •  Ctrl+S: save  •  v: toggle values  •  d: daemon  •  l: log  •  q: quit")
	s += "\n" + daemonStatusView(m)
	return appStyle.Render(s)
}

func renderProjectList(m model) string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Applications"))
	b.WriteString("\n\n")

	for i, project := range m.projects {
		cursor := "  "
		style := inactiveProjectStyle
		if m.focusPanel == 0 && i == m.selectedProject {
			cursor = "▸ "
			style = activeProjectStyle
		}
		b.WriteString(style.Render(cursor + project))
		b.WriteString("\n")
	}

	content := b.String()
	borderStyle := unfocusedBorderStyle
	if m.focusPanel == 0 {
		borderStyle = focusedBorderStyle
	}
	return borderStyle.Width(25).Render(content)
}

func renderSecretList(m model) string {
	var b strings.Builder
	project := m.projects[m.selectedProject]
	b.WriteString(headerStyle.Render(fmt.Sprintf("Secrets — %s", project)))
	b.WriteString("\n\n")

	for i, secret := range m.secrets {
		if m.editingSecret == i {
			b.WriteString(activeProjectStyle.Render(secret.key + " = "))
			b.WriteString(m.editInput.View())
			b.WriteString("  ")
			b.WriteString(helpStyle.Render("[Ctrl+S save  Esc cancel]"))
			b.WriteString("\n")
			continue
		}

		cursor := "  "
		style := secretKeyStyle
		if m.focusPanel == 1 && i == m.selectedSecret {
			cursor = "▸ "
			style = activeProjectStyle
		}

		keyPart := style.Render(cursor + secret.key + " = ")

		var valuePart string
		if !secret.set {
			valuePart = secretUnsetStyle.Render("not set")
		} else if m.showValues {
			valuePart = secretValueStyle.Render(secret.value)
		} else {
			valuePart = secretValueStyle.Render("••••••••")
		}

		b.WriteString(keyPart + valuePart + "\n")
	}

	if len(m.secrets) == 0 {
		b.WriteString(dimmedStyle.Render("  No secrets defined"))
		b.WriteString("\n")
	}

	content := b.String()
	borderStyle := unfocusedBorderStyle
	if m.focusPanel == 1 {
		borderStyle = focusedBorderStyle
	}
	panelWidth := m.width - 35
	if panelWidth < 30 {
		panelWidth = 30
	}
	return borderStyle.Width(panelWidth).Render(content)
}
