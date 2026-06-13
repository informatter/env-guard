package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/env-guard/env-guard/internal/vault"
)

func startRecovery(m model) (tea.Model, tea.Cmd) {
	ti := textinput.New()
	ti.Prompt = "▸ "
	ti.EchoMode = textinput.EchoPassword
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 40

	m.screen = screenRecovery
	m.recoveryStep = 0
	m.recoveryRootPw = ""
	m.recoveryNewPw = ""
	m.recoveryError = ""
	m.recoveryInput = ti
	return m, textinput.Blink
}

func updateRecovery(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	switch m.recoveryStep {
	case 0:
		return updateRecoveryRoot(msg, m)
	case 1:
		return updateRecoveryNewPw(msg, m)
	case 2:
		return updateRecoveryConfirm(msg, m)
	}
	return m, nil
}

func updateRecoveryRoot(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.recoveryInput, cmd = m.recoveryInput.Update(msg)

	if msg.String() == "enter" {
		rootPw := m.recoveryInput.Value()
		if rootPw == "" {
			m.recoveryError = "Password cannot be empty"
			return m, nil
		}

		if err := verifyRootPassword(rootPw); err != nil {
			m.recoveryError = "Wrong password"
			return m, nil
		}

		if err := m.vault.OpenWithRecovery(rootPw); err != nil {
			if err == vault.ErrNoVault {
				m.recoveryError = "Vault not found."
				return m, nil
			}
			m.recoveryError = "No recovery slot available."
			return m, nil
		}

		m.recoveryRootPw = rootPw
		m.recoveryStep = 1
		m.recoveryInput.SetValue("")
		m.recoveryInput.Reset()
		m.recoveryError = ""
		return m, textinput.Blink
	}

	if msg.String() == "esc" {
		m.screen = screenPassword
		return m, nil
	}

	return m, cmd
}

func updateRecoveryNewPw(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.recoveryInput, cmd = m.recoveryInput.Update(msg)

	if msg.String() == "enter" {
		newPw := m.recoveryInput.Value()
		if newPw == "" {
			m.recoveryError = "Password cannot be empty"
			return m, nil
		}
		m.recoveryNewPw = newPw
		m.recoveryStep = 2
		m.recoveryInput.SetValue("")
		m.recoveryInput.Reset()
		m.recoveryError = ""
		return m, textinput.Blink
	}

	if msg.String() == "esc" {
		m.recoveryStep = 0
		m.recoveryInput.SetValue("")
		m.recoveryInput.Reset()
		m.recoveryError = ""
		return m, nil
	}

	return m, cmd
}

func updateRecoveryConfirm(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.recoveryInput, cmd = m.recoveryInput.Update(msg)

	if msg.String() == "enter" {
		confirm := m.recoveryInput.Value()
		if confirm != m.recoveryNewPw {
			m.recoveryError = "Passwords do not match"
			m.recoveryStep = 1
			m.recoveryInput.SetValue("")
			m.recoveryInput.Reset()
			return m, textinput.Blink
		}

		if err := m.vault.ResetPassword(m.recoveryNewPw); err != nil {
			m.recoveryError = "Failed to reset password: " + err.Error()
			return m, nil
		}

		return loadDashboard(m)
	}

	if msg.String() == "esc" {
		m.recoveryStep = 1
		m.recoveryInput.SetValue("")
		m.recoveryInput.Reset()
		m.recoveryError = ""
		return m, nil
	}

	return m, cmd
}

func recoveryView(m model) string {
	s := logoStyle.Render("🔐 env-guard") + "\n\n"

	switch m.recoveryStep {
	case 0:
		s += headerStyle.Render("Password Recovery") + "\n\n"
		s += "Enter your system password to unlock the vault and reset your master password.\n\n"
		s += labelStyle.Render("System password:") + "\n"
		s += m.recoveryInput.View() + "\n"

	case 1:
		s += headerStyle.Render("Choose a New Master Password") + "\n\n"
		s += labelStyle.Render("New master password:") + "\n"
		s += m.recoveryInput.View() + "\n"

	case 2:
		s += headerStyle.Render("Confirm New Master Password") + "\n\n"
		s += labelStyle.Render("Enter the same password again:") + "\n"
		s += m.recoveryInput.View() + "\n"
	}

	if m.recoveryError != "" {
		s += "\n" + errorStyle.Render("✗ "+m.recoveryError) + "\n"
	}

	s += "\n" + helpStyle.Render("Enter to continue  •  Esc to go back")
	return appStyle.Render(s)
}
