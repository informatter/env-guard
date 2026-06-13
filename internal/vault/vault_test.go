package vault

import (
	"path/filepath"
	"testing"
)

func tempVault(t *testing.T) *SQLiteVault {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	return New(dbPath)
}

func TestCreateAndOpen(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("test-password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !v.IsOpen() {
		t.Fatal("expected vault to be open after Create")
	}
	v.Close()

	v2 := New(v.dbPath)
	if err := v2.Open("test-password"); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if !v2.IsOpen() {
		t.Fatal("expected vault to be open")
	}
	v2.Close()
}

func TestOpenWrongPassword(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("correct-password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	v.Close()

	v2 := New(v.dbPath)
	err := v2.Open("wrong-password")
	if err != ErrWrongPassword {
		t.Fatalf("expected ErrWrongPassword, got %v", err)
	}
}

func TestOpenNoVault(t *testing.T) {
	v := New(filepath.Join(t.TempDir(), "nonexistent.db"))
	err := v.Open("password")
	if err != ErrNoVault {
		t.Fatalf("expected ErrNoVault, got %v", err)
	}
}

func TestDoubleCreate(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	err := v.Create("password")
	if err != ErrVaultExists {
		t.Fatalf("expected ErrVaultExists, got %v", err)
	}
	v.Close()

	v2 := New(v.dbPath)
	v2.db = nil
	err = v2.Create("password")
	if err != ErrVaultExists {
		t.Fatalf("expected ErrVaultExists for existing file, got %v", err)
	}
}

func TestLock(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}
	if v.IsOpen() {
		t.Fatal("expected vault to be locked")
	}
	_, err := v.Projects()
	if err != ErrVaultLocked {
		t.Fatalf("expected ErrVaultLocked, got %v", err)
	}
}

func TestLockTwice(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("first Lock failed: %v", err)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("second Lock should be no-op: %v", err)
	}
}

func TestCreateProject(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := v.CreateProject("myapp"); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	projects, err := v.Projects()
	if err != nil {
		t.Fatalf("Projects failed: %v", err)
	}
	if len(projects) != 1 || projects[0] != "myapp" {
		t.Fatalf("expected [myapp], got %v", projects)
	}
}

func TestCreateProjectDuplicate(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := v.CreateProject("myapp"); err != nil {
		t.Fatalf("first CreateProject failed: %v", err)
	}
	if err := v.CreateProject("myapp"); err != nil {
		t.Fatalf("duplicate CreateProject should be no-op: %v", err)
	}
}

func TestMultipleProjects(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	for _, name := range []string{"app1", "app2", "app3"} {
		if err := v.CreateProject(name); err != nil {
			t.Fatalf("CreateProject(%q) failed: %v", name, err)
		}
	}
	projects, err := v.Projects()
	if err != nil {
		t.Fatalf("Projects failed: %v", err)
	}
	if len(projects) != 3 {
		t.Fatalf("expected 3 projects, got %d: %v", len(projects), projects)
	}
}

func TestInitSecret(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	v.CreateProject("myapp")
	if err := v.InitSecret("myapp", "DATABASE_URL"); err != nil {
		t.Fatalf("InitSecret failed: %v", err)
	}
	keys, err := v.SecretKeys("myapp")
	if err != nil {
		t.Fatalf("SecretKeys failed: %v", err)
	}
	if len(keys) != 1 || keys[0] != "DATABASE_URL" {
		t.Fatalf("expected [DATABASE_URL], got %v", keys)
	}
	_, err = v.GetSecret("myapp", "DATABASE_URL")
	if err != ErrSecretNotSet {
		t.Fatalf("expected ErrSecretNotSet, got %v", err)
	}
}

func TestSetAndGetSecret(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	v.CreateProject("myapp")
	if err := v.SetSecret("myapp", "DATABASE_URL", "postgres://localhost:5432/db"); err != nil {
		t.Fatalf("SetSecret failed: %v", err)
	}
	val, err := v.GetSecret("myapp", "DATABASE_URL")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if val != "postgres://localhost:5432/db" {
		t.Fatalf("expected 'postgres://localhost:5432/db', got %q", val)
	}
}

func TestSetSecretCreatesIfNotExists(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	v.CreateProject("myapp")
	if err := v.SetSecret("myapp", "API_KEY", "sk-1234"); err != nil {
		t.Fatalf("SetSecret failed: %v", err)
	}
	val, err := v.GetSecret("myapp", "API_KEY")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if val != "sk-1234" {
		t.Fatalf("expected 'sk-1234', got %q", val)
	}
}

func TestGetSecretNotFound(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	v.CreateProject("myapp")
	_, err := v.GetSecret("myapp", "NONEXISTENT")
	if err != ErrSecretNotFound {
		t.Fatalf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestGetSecretProjectNotFound(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	_, err := v.GetSecret("nonexistent", "KEY")
	if err != ErrProjectNotFound {
		t.Fatalf("expected ErrProjectNotFound, got %v", err)
	}
}

func TestGetSecrets(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	v.CreateProject("myapp")
	v.SetSecret("myapp", "DATABASE_URL", "postgres://db")
	v.SetSecret("myapp", "API_KEY", "sk-1234")
	secrets, err := v.GetSecrets("myapp")
	if err != nil {
		t.Fatalf("GetSecrets failed: %v", err)
	}
	if len(secrets) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(secrets))
	}
	if secrets["DATABASE_URL"] != "postgres://db" {
		t.Fatalf("unexpected DATABASE_URL: %q", secrets["DATABASE_URL"])
	}
	if secrets["API_KEY"] != "sk-1234" {
		t.Fatalf("unexpected API_KEY: %q", secrets["API_KEY"])
	}
}

func TestGetSecretsSkipsUnset(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	v.CreateProject("myapp")
	v.InitSecret("myapp", "UNSET_SECRET")
	v.SetSecret("myapp", "SET_SECRET", "value")
	secrets, err := v.GetSecrets("myapp")
	if err != nil {
		t.Fatalf("GetSecrets failed: %v", err)
	}
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret (unset skipped), got %d", len(secrets))
	}
	if secrets["SET_SECRET"] != "value" {
		t.Fatalf("unexpected value: %q", secrets["SET_SECRET"])
	}
}

func TestUpdateSecret(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	v.CreateProject("myapp")
	v.SetSecret("myapp", "KEY", "original")
	v.SetSecret("myapp", "KEY", "updated")
	val, err := v.GetSecret("myapp", "KEY")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if val != "updated" {
		t.Fatalf("expected 'updated', got %q", val)
	}
}

func TestAccessLog(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := v.LogAccess("myapp", "/usr/bin/myapp", "DATABASE_URL,API_KEY", 12345); err != nil {
		t.Fatalf("LogAccess failed: %v", err)
	}
	entries, err := v.AccessLog()
	if err != nil {
		t.Fatalf("AccessLog failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.AppName != "myapp" {
		t.Fatalf("expected app myapp, got %q", e.AppName)
	}
	if e.ProcessPath != "/usr/bin/myapp" {
		t.Fatalf("expected /usr/bin/myapp, got %q", e.ProcessPath)
	}
	if e.PID != 12345 {
		t.Fatalf("expected pid 12345, got %d", e.PID)
	}
	if e.Timestamp == 0 {
		t.Fatal("expected non-zero timestamp")
	}
}

func TestAccessLogLocked(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	v.Lock()
	err := v.LogAccess("myapp", "/path", "KEY", 1)
	if err != ErrVaultLocked {
		t.Fatalf("expected ErrVaultLocked, got %v", err)
	}
}

func TestSecretKeys(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	v.CreateProject("myapp")
	v.InitSecret("myapp", "B_KEY")
	v.InitSecret("myapp", "A_KEY")
	v.InitSecret("myapp", "C_KEY")
	keys, err := v.SecretKeys("myapp")
	if err != nil {
		t.Fatalf("SecretKeys failed: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "A_KEY" || keys[1] != "B_KEY" || keys[2] != "C_KEY" {
		t.Fatalf("expected sorted keys, got %v", keys)
	}
}

func TestRoundTripPersistence(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("mypassword"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	v.CreateProject("production")
	v.SetSecret("production", "DB_PASS", "s3cret!")
	v.Close()

	v2 := New(v.dbPath)
	if err := v2.Open("mypassword"); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	val, err := v2.GetSecret("production", "DB_PASS")
	if err != nil {
		t.Fatalf("GetSecret after reopen failed: %v", err)
	}
	if val != "s3cret!" {
		t.Fatalf("expected 's3cret!', got %q", val)
	}
	v2.Close()
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	nonce := make([]byte, 12)
	for i := range nonce {
		nonce[i] = byte(i + 0x10)
	}

	plaintext := []byte("hello world")
	ciphertext, err := encryptSecretValue(plaintext, key, nonce)
	if err != nil {
		t.Fatalf("encryptSecretValue failed: %v", err)
	}

	decrypted, err := decryptSecretValue(ciphertext, key, nonce)
	if err != nil {
		t.Fatalf("decryptSecretValue failed: %v", err)
	}

	if string(decrypted) != "hello world" {
		t.Fatalf("expected 'hello world', got %q", string(decrypted))
	}
}

func TestEncryptWrongKey(t *testing.T) {
	key := make([]byte, 32)
	nonce := make([]byte, 12)

	ciphertext, err := encryptSecretValue([]byte("secret"), key, nonce)
	if err != nil {
		t.Fatalf("encryptSecretValue failed: %v", err)
	}

	wrongKey := make([]byte, 32)
	wrongKey[0] = 0xFF
	_, err = decryptSecretValue(ciphertext, wrongKey, nonce)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestOperationsOnLockedVault(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	v.Lock()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"CreateProject", func() error { return v.CreateProject("x") }},
		{"Projects", func() error { _, err := v.Projects(); return err }},
		{"InitSecret", func() error { return v.InitSecret("x", "y") }},
		{"SetSecret", func() error { return v.SetSecret("x", "y", "z") }},
		{"GetSecret", func() error { _, err := v.GetSecret("x", "y"); return err }},
		{"GetSecrets", func() error { _, err := v.GetSecrets("x"); return err }},
		{"SecretKeys", func() error { _, err := v.SecretKeys("x"); return err }},
		{"AccessLog", func() error { _, err := v.AccessLog(); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err != ErrVaultLocked {
				t.Fatalf("expected ErrVaultLocked, got %v", err)
			}
		})
	}
}

func TestDefaultVaultPath(t *testing.T) {
	path, err := DefaultVaultPath()
	if err != nil {
		t.Fatalf("DefaultVaultPath failed: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestSetRecoverySlot(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("master-pw"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := v.SetRecoverySlot("root-pw"); err != nil {
		t.Fatalf("SetRecoverySlot: %v", err)
	}

	if err := v.SetRecoverySlot("another-root"); err == nil {
		t.Fatal("expected error for duplicate recovery slot")
	}

	v.Close()
}

func TestHasRecoverySlot(t *testing.T) {
	t.Run("no recovery slot", func(t *testing.T) {
		v := tempVault(t)
		if err := v.Create("pw"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		v.Close()

		if v.HasRecoverySlot() {
			t.Fatal("expected HasRecoverySlot to be false")
		}
	})

	t.Run("with recovery slot", func(t *testing.T) {
		v := tempVault(t)
		if err := v.Create("pw"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := v.SetRecoverySlot("root-pw"); err != nil {
			t.Fatalf("SetRecoverySlot: %v", err)
		}
		v.Close()

		if !v.HasRecoverySlot() {
			t.Fatal("expected HasRecoverySlot to be true")
		}
	})

	t.Run("works when locked", func(t *testing.T) {
		v := tempVault(t)
		if err := v.Create("pw"); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := v.SetRecoverySlot("root-pw"); err != nil {
			t.Fatalf("SetRecoverySlot: %v", err)
		}
		v.Close()

		v2 := New(v.dbPath)
		if !v2.HasRecoverySlot() {
			t.Fatal("expected HasRecoverySlot to work on locked vault")
		}
	})
}

func TestOpenWithRecovery(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("master-pw"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := v.SetRecoverySlot("root-pw"); err != nil {
		t.Fatalf("SetRecoverySlot: %v", err)
	}
	v.Close()

	v2 := New(v.dbPath)
	if err := v2.OpenWithRecovery("root-pw"); err != nil {
		t.Fatalf("OpenWithRecovery: %v", err)
	}
	if !v2.IsOpen() {
		t.Fatal("expected vault to be open")
	}

	v2.Close()
}

func TestOpenWithRecoveryWrongPassword(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("master-pw"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := v.SetRecoverySlot("root-pw"); err != nil {
		t.Fatalf("SetRecoverySlot: %v", err)
	}
	v.Close()

	v2 := New(v.dbPath)
	err := v2.OpenWithRecovery("wrong-root-pw")
	if err != ErrWrongPassword {
		t.Fatalf("expected ErrWrongPassword, got %v", err)
	}
}

func TestOpenWithRecoveryNoSlot(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("pw"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	v.Close()

	v2 := New(v.dbPath)
	err := v2.OpenWithRecovery("root-pw")
	if err == nil {
		t.Fatal("expected error for missing recovery slot")
	}
}

func TestFullRecoveryFlow(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("master-pw"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := v.SetRecoverySlot("root-pw"); err != nil {
		t.Fatalf("SetRecoverySlot: %v", err)
	}
	v.Close()

	v2 := New(v.dbPath)
	if err := v2.OpenWithRecovery("root-pw"); err != nil {
		t.Fatalf("OpenWithRecovery: %v", err)
	}

	if err := v2.ResetPassword("new-master-pw"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	v2.Close()

	v3 := New(v.dbPath)
	if err := v3.Open("new-master-pw"); err != nil {
		t.Fatalf("Open with new password: %v", err)
	}
	v3.Close()

	v4 := New(v.dbPath)
	if err := v4.Open("master-pw"); err == nil {
		t.Fatal("expected old password to no longer work")
	}
	v4.Close()
}

func TestResetPassword(t *testing.T) {
	v := tempVault(t)
	if err := v.Create("old-pw"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := v.CreateProject("testapp"); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := v.SetSecret("testapp", "KEY", "secret-value"); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	v.Close()

	v2 := New(v.dbPath)
	if err := v2.Open("old-pw"); err != nil {
		t.Fatalf("Open with old password: %v", err)
	}

	if err := v2.ResetPassword("new-pw"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	v2.Close()

	v3 := New(v.dbPath)
	if err := v3.Open("new-pw"); err != nil {
		t.Fatalf("Open with new password: %v", err)
	}

	val, err := v3.GetSecret("testapp", "KEY")
	if err != nil {
		t.Fatalf("GetSecret after reset: %v", err)
	}
	if val != "secret-value" {
		t.Fatalf("expected 'secret-value', got %q", val)
	}
	v3.Close()
}
