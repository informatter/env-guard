package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/env-guard/env-guard/internal/vault"
)

type vaultOpenedMsg struct{}

func updatePassword(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)

	if msg.String() == "enter" {
		password := m.passwordInput.Value()
		if len(password) == 0 {
			m.passwordError = "Password cannot be empty"
			return m, nil
		}
		return unlockVault(m, password)
	}

	return m, cmd
}

func unlockVault(m model, password string) (tea.Model, tea.Cmd) {
	m.password = password
	m.passwordInput.Blur()

	if err := m.vault.Open(password); err != nil {
		m.passwordInput.SetValue("")
		m.passwordInput.Reset()
		m.passwordInput.Focus()
		switch err {
		case vault.ErrWrongPassword:
			m.passwordError = "Wrong password"
		case vault.ErrNoVault:
			m.passwordError = "Vault not found"
		default:
			m.passwordError = err.Error()
		}
		return m, nil
	}

	return m, tea.Batch(textinput.Blink, func() tea.Msg {
		return vaultOpenedMsg{}
	})
}

func passwordView(m model) string {
	s := logoStyle.Render("🔐 env-guard") + "\n\n"
	s += headerStyle.Render("Unlock Vault") + "\n\n"
	s += "Enter your master password to unlock the vault.\n\n"

	if m.passwordError != "" {
		s += errorStyle.Render("✗ "+m.passwordError) + "\n\n"
	}

	s += labelStyle.Render("Master password:") + "\n"
	s += m.passwordInput.View() + "\n\n"
	s += helpStyle.Render("Enter to unlock  •  Ctrl+C to quit") + "\n"
	return appStyle.Render(s)
}
