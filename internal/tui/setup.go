package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type vaultCreatedMsg struct{}

func updateSetup(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	switch m.setupStep {
	case 0:
		return updateSetupWelcome(msg, m)
	case 1:
		return updateSetupPassword(msg, m)
	case 2:
		return updateSetupConfirm(msg, m)
	case 3:
		return updateSetupDone(msg, m)
	}
	return m, nil
}

func updateSetupWelcome(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		m.setupStep = 1
		m.setupInput.Focus()
		m.setupInput.EchoMode = textinput.EchoPassword
		return m, nil
	}
	return m, nil
}

func updateSetupPassword(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.setupInput, cmd = m.setupInput.Update(msg)

	if msg.String() == "enter" {
		password := m.setupInput.Value()
		if len(password) == 0 {
			m.setupError = "Password cannot be empty"
			return m, nil
		}
		m.setupPassword = password
		m.setupInput.SetValue("")
		m.setupInput.Reset()
		m.setupStep = 2
		m.setupError = ""
		return m, nil
	}

	if msg.String() == "esc" {
		m.setupStep = 0
		m.setupInput.SetValue("")
		m.setupError = ""
		return m, nil
	}

	return m, cmd
}

func updateSetupConfirm(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.setupInput, cmd = m.setupInput.Update(msg)

	if msg.String() == "enter" {
		confirm := m.setupInput.Value()
		if confirm != m.setupPassword {
			m.setupError = "Passwords do not match"
			m.setupInput.SetValue("")
			m.setupStep = 1
			return m, nil
		}
		return createVault(m)
	}

	if msg.String() == "esc" {
		m.setupStep = 1
		m.setupInput.SetValue("")
		m.setupInput.Reset()
		m.setupError = ""
		return m, nil
	}

	return m, cmd
}

func updateSetupDone(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		if m.hasConfig {
			return loadDashboard(m)
		}
		return loadDashboard(m)
	}
	return m, nil
}

func createVault(m model) (tea.Model, tea.Cmd) {
	m.setupStep = 3
	m.setupError = ""
	m.creating = true
	m.setupInput.Blur()

	if err := m.vault.Create(m.setupPassword); err != nil {
		m.creating = false
		m.err = err
		return m, nil
	}

	if m.cfg != nil {
		for _, name := range m.cfg.AppNames() {
			if err := m.vault.CreateProject(name); err != nil {
				m.creating = false
				m.err = err
				return m, nil
			}
			app := m.cfg.Applications[name]
			for _, secret := range app.Secrets {
				if err := m.vault.InitSecret(name, secret); err != nil {
					m.creating = false
					m.err = err
					return m, nil
				}
			}
		}
	}

	m.creating = false
	return m, tea.Batch(textinput.Blink, func() tea.Msg {
		return vaultCreatedMsg{}
	})
}

func setupView(m model) string {
	switch m.setupStep {
	case 0:
		return setupViewWelcome(m)
	case 1:
		return setupViewPassword(m)
	case 2:
		return setupViewConfirm(m)
	case 3:
		return setupViewCreating(m)
	}
	return ""
}

func setupViewWelcome(m model) string {
	s := logoStyle.Render("🔐 env-guard") + "\n\n"

	if m.hasConfig {
		totalSecrets := 0
		for _, app := range m.cfg.Applications {
			totalSecrets += len(app.Secrets)
		}
		s += headerStyle.Render("Welcome!") + "\n\n"
		s += fmt.Sprintf("Found %s with %d apps and %d secrets.\n\n",
			labelStyle.Render("env-guard.yaml"),
			len(m.cfg.Applications),
			totalSecrets,
		)
		s += "Let's create an encrypted vault to store your secrets securely.\n\n"
	} else {
		s += headerStyle.Render("Welcome!") + "\n\n"
		s += "No " + labelStyle.Render("env-guard.yaml") + " found.\n"
		s += helpStyle.Render("Create one to define your apps and secrets.") + "\n\n"
		s += "You can continue with an empty vault and add a config later.\n\n"
	}

	s += helpStyle.Render("Press Enter to begin") + "\n"
	return appStyle.Render(s)
}

func setupViewPassword(m model) string {
	s := logoStyle.Render("🔐 env-guard") + "\n\n"
	s += headerStyle.Render("Step 1: Choose a Master Password") + "\n\n"
	s += "This password encrypts all your secrets. " +
		helpStyle.Render("Don't forget it — there's no recovery without it.") + "\n\n"
	s += labelStyle.Render("Master password:") + "\n"
	s += m.setupInput.View() + "\n"

	if m.setupError != "" {
		s += "\n" + errorStyle.Render("✗ "+m.setupError) + "\n"
	}

	s += "\n" + helpStyle.Render("Enter to continue  •  Esc to go back") + "\n"
	return appStyle.Render(s)
}

func setupViewConfirm(m model) string {
	s := logoStyle.Render("🔐 env-guard") + "\n\n"
	s += headerStyle.Render("Step 2: Confirm Password") + "\n\n"
	s += labelStyle.Render("Enter the same password again:") + "\n"
	s += m.setupInput.View() + "\n"

	if m.setupError != "" {
		s += "\n" + errorStyle.Render("✗ "+m.setupError) + "\n"
	}

	s += "\n" + helpStyle.Render("Enter to confirm  •  Esc to go back") + "\n"
	return appStyle.Render(s)
}

func setupViewCreating(m model) string {
	s := logoStyle.Render("🔐 env-guard") + "\n\n"
	s += "Creating your vault...\n\n"
	if m.hasConfig {
		s += fmt.Sprintf("  • %d application(s)\n", len(m.cfg.Applications))
		total := 0
		for _, app := range m.cfg.Applications {
			total += len(app.Secrets)
		}
		s += fmt.Sprintf("  • %d secret(s)\n", total)
	}
	return appStyle.Render(s)
}
