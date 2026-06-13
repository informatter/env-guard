package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/env-guard/env-guard/internal/config"
	"github.com/env-guard/env-guard/internal/vault"
)

func mockPAM() func() {
	orig := verifyRootPassword
	verifyRootPassword = func(password string) error { return nil }
	return func() { verifyRootPassword = orig }
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
		vaultPath:  filepath.Join(t.TempDir(), "vault.db"),
		vault:      vault.New(filepath.Join(t.TempDir(), "vault.db")),
		screen:     screenSetup,
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
		vaultPath:     dbPath,
		vault:         vault.New(dbPath),
		screen:        screenPassword,
		passwordInput: ti,
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
		vaultPath:     dbPath,
		vault:         v2,
		cfg:           cfg,
		hasConfig:     true,
		screen:        screenPassword,
		passwordInput: ti,
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

func TestDaemonToggleStartStop(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	v := vault.New(dbPath)
	if err := v.Create("pw"); err != nil {
		t.Fatal(err)
	}
	v.CreateProject("testapp")
	v.Close()

	v2 := vault.New(dbPath)
	if err := v2.Open("pw"); err != nil {
		t.Fatal(err)
	}

	m := model{
		vaultPath:        dbPath,
		vault:            v2,
		screen:           screenDashboard,
		projects:         []string{"testapp"},
		selectedProject:  0,
		focusPanel:       0,
		editingSecret:    -1,
		daemonSocketPath: filepath.Join(dir, "test.sock"),
	}

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m2 := res.(model)
	if !m2.daemonRunning {
		t.Fatal("expected daemon to start")
	}
	if m2.daemonError != "" {
		t.Fatalf("unexpected daemon error: %s", m2.daemonError)
	}

	res, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m3 := res.(model)
	if m3.daemonRunning {
		t.Fatal("expected daemon to stop")
	}

	v2.Close()
}

func TestDaemonStatusView(t *testing.T) {
	t.Run("stopped", func(t *testing.T) {
		m := model{}
		v := daemonStatusView(m)
		if !strings.Contains(v, "stopped") {
			t.Fatalf("expected 'stopped' in status view, got: %s", v)
		}
	})

	t.Run("error", func(t *testing.T) {
		m := model{daemonError: "connection refused"}
		v := daemonStatusView(m)
		if !strings.Contains(v, "connection refused") {
			t.Fatalf("expected error in status view, got: %s", v)
		}
	})
}

func TestAccessLogOpen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	v := vault.New(dbPath)
	if err := v.Create("pw"); err != nil {
		t.Fatal(err)
	}
	if err := v.LogAccess("testapp", "/usr/bin/test", "DATABASE_URL", 12345); err != nil {
		t.Fatal(err)
	}
	if err := v.LogAccess("otherapp", "/usr/bin/other", "API_KEY", 67890); err != nil {
		t.Fatal(err)
	}
	v.Close()

	v2 := vault.New(dbPath)
	if err := v2.Open("pw"); err != nil {
		t.Fatal(err)
	}

	m := model{
		vaultPath: dbPath,
		vault:     v2,
		screen:    screenDashboard,
	}

	res, _ := openAccessLog(m)
	m2 := res.(model)

	if m2.screen != screenAccessLog {
		t.Fatalf("expected screenAccessLog, got %d", m2.screen)
	}
	if len(m2.accessLog) != 2 {
		t.Fatalf("expected 2 access log entries, got %d", len(m2.accessLog))
	}
	seen := map[string]bool{}
	for _, e := range m2.accessLog {
		seen[e.AppName] = true
	}
	if !seen["testapp"] || !seen["otherapp"] {
		t.Fatalf("expected both apps in access log, got %+v", m2.accessLog)
	}

	v2.Close()
}

func TestAccessLogView(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		m := model{
			screen:    screenAccessLog,
			accessLog: []vault.AccessEntry{},
		}
		v := accessLogView(m)
		if !strings.Contains(v, "No access log entries") {
			t.Fatalf("expected empty message, got: %s", v)
		}
	})

	t.Run("with entries", func(t *testing.T) {
		m := model{
			screen: screenAccessLog,
			accessLog: []vault.AccessEntry{
				{ID: 2, Timestamp: 2000, AppName: "app2", ProcessPath: "/bin/app2", KeysRequested: "KEY2", PID: 200},
				{ID: 1, Timestamp: 1000, AppName: "app1", ProcessPath: "/bin/app1", KeysRequested: "KEY1", PID: 100},
			},
			height: 30,
		}
		v := accessLogView(m)
		if !strings.Contains(v, "app2") || !strings.Contains(v, "app1") {
			t.Fatalf("expected both entries in view, got: %s", v)
		}
		if !strings.Contains(v, "Access Log") {
			t.Fatalf("expected title, got: %s", v)
		}
	})

	t.Run("scroll", func(t *testing.T) {
		entries := make([]vault.AccessEntry, 20)
		for i := 0; i < 20; i++ {
			entries[i] = vault.AccessEntry{
				ID: int64(i), Timestamp: int64(1000 + i),
				AppName: "app", ProcessPath: "/bin/app", KeysRequested: "KEY", PID: 100 + i,
			}
		}
		m := model{
			screen:          screenAccessLog,
			accessLog:       entries,
			accessLogScroll: 5,
			height:          30,
		}
		v := accessLogView(m)
		if !strings.Contains(v, "6") {
			t.Fatalf("expected scroll indicator showing range, got: %s", v)
		}
	})
}

func TestAccessLogEscReturnsToDashboard(t *testing.T) {
	m := model{
		screen:    screenAccessLog,
		accessLog: []vault.AccessEntry{},
	}

	res, _ := updateAccessLog(tea.KeyMsg{Type: tea.KeyEsc}, m)
	m2 := res.(model)
	if m2.screen != screenDashboard {
		t.Fatalf("expected screenDashboard after Esc, got %d", m2.screen)
	}
}

func TestAccessLogScroll(t *testing.T) {
	entries := make([]vault.AccessEntry, 10)
	for i := 0; i < 10; i++ {
		entries[i] = vault.AccessEntry{ID: int64(i), AppName: "app"}
	}
	m := model{
		screen:          screenAccessLog,
		accessLog:       entries,
		accessLogScroll: 5,
	}

	res, _ := updateAccessLog(tea.KeyMsg{Type: tea.KeyDown}, m)
	m2 := res.(model)
	if m2.accessLogScroll != 6 {
		t.Fatalf("expected scroll 6 after down, got %d", m2.accessLogScroll)
	}

	res, _ = m2.Update(tea.KeyMsg{Type: tea.KeyUp})
	m3 := res.(model)
	if m3.accessLogScroll != 5 {
		t.Fatalf("expected scroll 5 after up, got %d", m3.accessLogScroll)
	}
}

func TestPasswordScreenRecoveryTrigger(t *testing.T) {
	restore := mockPAM()
	defer restore()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	v := vault.New(dbPath)
	v.Create("pw")
	v.Close()

	v2 := vault.New(dbPath)

	m := model{
		vaultPath:     dbPath,
		vault:         v2,
		screen:        screenPassword,
		passwordInput: textinput.New(),
	}

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m2 := res.(model)
	if m2.screen != screenRecovery {
		t.Fatalf("expected screenRecovery, got %d", m2.screen)
	}
	if m2.recoveryStep != 0 {
		t.Fatalf("expected recoveryStep 0, got %d", m2.recoveryStep)
	}
}

func TestRecoveryWrongSystemPassword(t *testing.T) {
	restore := mockPAM()
	defer restore()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	v := vault.New(dbPath)
	v.Create("pw")
	v.Close()

	m := model{
		vaultPath:     dbPath,
		vault:         vault.New(dbPath),
		screen:        screenRecovery,
		recoveryStep:  0,
		recoveryInput: textinput.New(),
	}
	m.recoveryInput.Focus()
	m.recoveryInput.SetValue("wrong-pw")

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := res.(model)
	if m2.screen != screenRecovery {
		t.Fatalf("expected to stay on recovery screen, got %d", m2.screen)
	}
	if m2.recoveryError == "" {
		t.Fatal("expected error for wrong system password")
	}
}

func TestRecoveryEscReturnsToPassword(t *testing.T) {
	m := model{
		screen:        screenRecovery,
		recoveryStep:  0,
		recoveryInput: textinput.New(),
	}

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := res.(model)
	if m2.screen != screenPassword {
		t.Fatalf("expected screenPassword after Esc, got %d", m2.screen)
	}
}

func TestRecoveryFullFlow(t *testing.T) {
	restore := mockPAM()
	defer restore()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	v := vault.New(dbPath)
	v.Create("old-pw")
	v.SetRecoverySlot("root-pw")
	v.Close()

	m := model{
		vaultPath:     dbPath,
		vault:         vault.New(dbPath),
		screen:        screenRecovery,
		recoveryStep:  0,
		recoveryInput: textinput.New(),
	}
	m.recoveryInput.Focus()
	m.recoveryInput.SetValue("root-pw")

	// Step 0: Enter root password (PAM mocked to always succeed)
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := res.(model)
	if m2.recoveryStep != 1 {
		t.Fatalf("expected recoveryStep 1 after root pw, got %d", m2.recoveryStep)
	}

	// Step 1: Enter new master password
	m2.recoveryInput.Focus()
	m2.recoveryInput.SetValue("new-pw")
	res2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := res2.(model)
	if m3.recoveryStep != 2 {
		t.Fatalf("expected recoveryStep 2 after new pw, got %d", m3.recoveryStep)
	}

	// Step 2: Confirm new master password
	m3.recoveryInput.SetValue("new-pw")
	res3, _ := m3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m4 := res3.(model)
	if m4.screen != screenDashboard {
		t.Fatalf("expected screenDashboard after recovery, got %d", m4.screen)
	}

	// Verify new password works and old password does not
	m4.vault.Close()

	v3 := vault.New(dbPath)
	if err := v3.Open("new-pw"); err != nil {
		t.Fatalf("new password should work after recovery: %v", err)
	}
	v3.Close()

	v4 := vault.New(dbPath)
	if err := v4.Open("old-pw"); err == nil {
		t.Fatal("old password should not work after recovery")
	}
	v4.Close()
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
