package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/env-guard/env-guard/internal/config"
	"github.com/env-guard/env-guard/internal/vault"
)

func tempVault(t *testing.T) *vault.SQLiteVault {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	return vault.New(dbPath)
}

func writeConfig(t *testing.T, dir string) *config.Config {
	t.Helper()
	yaml := `
applications:
  myapp:
    secrets:
      - DATABASE_URL
      - API_KEY
`
	path := filepath.Join(dir, "env-guard.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestModelInit_NoVaultNoConfig(t *testing.T) {
	ti := textinput.New()
	m := model{
		vaultPath: filepath.Join(t.TempDir(), "vault.db"),
		vault:     vault.New(filepath.Join(t.TempDir(), "vault.db")),
		screen:    screenSetup,
		setupInput: ti,
	}

	if m.screen != screenSetup {
		t.Fatalf("expected screenSetup, got %d", m.screen)
	}
}

func TestModelInit_VaultExists(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	v := vault.New(dbPath)
	if err := v.Create("password"); err != nil {
		t.Fatal(err)
	}
	v.Close()

	ti := textinput.New()
	m := model{
		vaultPath:      dbPath,
		vault:          vault.New(dbPath),
		screen:         screenPassword,
		passwordInput:  ti,
	}

	if m.screen != screenPassword {
		t.Fatalf("expected screenPassword, got %d", m.screen)
	}
}

func TestSetupToDashboardFlow(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir)

	dbPath := filepath.Join(dir, "vault.db")
	v := vault.New(dbPath)
	if err := v.Create("test-password"); err != nil {
		t.Fatal(err)
	}
	for _, name := range cfg.AppNames() {
		if err := v.CreateProject(name); err != nil {
			t.Fatal(err)
		}
		app := cfg.Applications[name]
		for _, secret := range app.Secrets {
			if err := v.InitSecret(name, secret); err != nil {
				t.Fatal(err)
			}
		}
	}
	v.Close()

	v2 := vault.New(dbPath)
	if err := v2.Open("test-password"); err != nil {
		t.Fatal(err)
	}

	ti := textinput.New()
	m := model{
		vaultPath:      dbPath,
		vault:          v2,
		cfg:            cfg,
		hasConfig:      true,
		screen:         screenPassword,
		passwordInput:  ti,
	}

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(msg)
	m2 := result.(model)

	if m2.screen != screenPassword {
		t.Fatalf("expected to stay on password screen (empty password), got screen %d", m2.screen)
	}
}

func TestSecretSavedMessage(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	v := vault.New(dbPath)
	if err := v.Create("pw"); err != nil {
		t.Fatal(err)
	}
	v.CreateProject("testapp")
	v.InitSecret("testapp", "MY_KEY")
	v.Close()

	v2 := vault.New(dbPath)
	v2.Open("pw")

	m := model{
		vaultPath:       dbPath,
		vault:           v2,
		cfg:             nil,
		hasConfig:       false,
		screen:          screenDashboard,
		projects:        []string{"testapp"},
		selectedProject: 0,
		selectedSecret:  0,
		focusPanel:      0,
		editingSecret:   -1,
		showValues:      false,
		savedMessage:    "",
	}

	secrets, err := v2.SecretKeys("testapp")
	if err != nil {
		t.Fatal(err)
	}
	m.secrets = make([]secretItem, len(secrets))
	for i, key := range secrets {
		m.secrets[i] = secretItem{key: key, set: false}
	}

	if err := v2.SetSecret("testapp", "MY_KEY", "my-value"); err != nil {
		t.Fatal(err)
	}
	m.savedMessage = "✓ Saved!"

	if m.savedMessage != "✓ Saved!" {
		t.Fatalf("expected saved message, got %q", m.savedMessage)
	}

	v2.Close()
}

func TestNavigateBetweenSecrets(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	v := vault.New(dbPath)
	if err := v.Create("pw"); err != nil {
		t.Fatal(err)
	}
	v.CreateProject("testapp")
	v.InitSecret("testapp", "KEY_A")
	v.InitSecret("testapp", "KEY_B")
	v.InitSecret("testapp", "KEY_C")
	v.Close()

	v2 := vault.New(dbPath)
	if err := v2.Open("pw"); err != nil {
		t.Fatal(err)
	}

	m := model{
		vaultPath:       dbPath,
		vault:           v2,
		screen:          screenDashboard,
		projects:        []string{"testapp"},
		selectedProject: 0,
		selectedSecret:  0,
		focusPanel:      0,
		editingSecret:   -1,
	}
	res, _ := loadSecrets(m)
	cur := res.(model)

	if cur.selectedSecret != 0 {
		t.Fatalf("expected selectedSecret=0, got %d", cur.selectedSecret)
	}

	res2, _ := cur.Update(tea.KeyMsg{Type: tea.KeyTab})
	cur = res2.(model)
	if cur.focusPanel != 1 {
		t.Fatalf("expected focusPanel=1 after tab, got %d", cur.focusPanel)
	}

	res2, _ = cur.Update(tea.KeyMsg{Type: tea.KeyDown})
	cur = res2.(model)
	if cur.selectedSecret != 1 {
		t.Fatalf("expected selectedSecret=1 after down, got %d", cur.selectedSecret)
	}

	res2, _ = cur.Update(tea.KeyMsg{Type: tea.KeyDown})
	cur = res2.(model)
	if cur.selectedSecret != 2 {
		t.Fatalf("expected selectedSecret=2, got %d", cur.selectedSecret)
	}

	res2, _ = cur.Update(tea.KeyMsg{Type: tea.KeyUp})
	cur = res2.(model)
	if cur.selectedSecret != 1 {
		t.Fatalf("expected selectedSecret=1 after up, got %d", cur.selectedSecret)
	}

	v2.Close()
}

func TestNavigateBetweenSecretsAfterSave(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	v := vault.New(dbPath)
	if err := v.Create("pw"); err != nil {
		t.Fatal(err)
	}
	v.CreateProject("testapp")
	v.InitSecret("testapp", "KEY_A")
	v.InitSecret("testapp", "KEY_B")
	v.Close()

	v2 := vault.New(dbPath)
	if err := v2.Open("pw"); err != nil {
		t.Fatal(err)
	}

	m := model{
		vaultPath:       dbPath,
		vault:           v2,
		screen:          screenDashboard,
		projects:        []string{"testapp"},
		selectedProject: 0,
		selectedSecret:  0,
		focusPanel:      1,
		editingSecret:   -1,
	}
	res, _ := loadSecrets(m)
	m2 := res.(model)

	// Press Enter to start editing secret 0
	res2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := res2.(model)
	if m3.editingSecret != 0 {
		t.Fatalf("expected editingSecret=0 after enter, got %d", m3.editingSecret)
	}

	// Press Ctrl+S to save
	res3, _ := m3.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m4 := res3.(model)
	if m4.editingSecret != -1 {
		t.Fatalf("expected editingSecret=-1 after save, got %d", m4.editingSecret)
	}

	// Verify we can still navigate
	res4, _ := m4.Update(tea.KeyMsg{Type: tea.KeyDown})
	m5 := res4.(model)
	if m5.selectedSecret != 1 {
		t.Fatalf("expected selectedSecret=1 after down, got %d", m5.selectedSecret)
	}

	v2.Close()
}
