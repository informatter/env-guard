package daemon

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/env-guard/env-guard/internal/vault"
)

type mockVault struct {
	secrets       map[string]map[string]string
	logApp        string
	logPath       string
	logKeys       string
	logPID        int
	logged        bool
	getSecretsErr error
}

func (m *mockVault) GetSecrets(project string) (map[string]string, error) {
	if m.getSecretsErr != nil {
		return nil, m.getSecretsErr
	}
	s, ok := m.secrets[project]
	if !ok {
		return map[string]string{}, nil
	}
	return s, nil
}

func (m *mockVault) LogAccess(appName, processPath, keysRequested string, pid int) error {
	m.logApp = appName
	m.logPath = processPath
	m.logKeys = keysRequested
	m.logPID = pid
	m.logged = true
	return nil
}

type mockConfig struct {
	apps map[string][]string
}

func (m *mockConfig) HasApp(appName string) bool {
	_, ok := m.apps[appName]
	return ok
}

func (m *mockConfig) AllowedPaths(appName string) []string {
	return m.apps[appName]
}

func testCredCtx(t *testing.T) context.Context {
	t.Helper()
	startTime, err := readProcStartTime(os.Getpid())
	if err != nil {
		t.Fatalf("readProcStartTime: %v", err)
	}
	return context.WithValue(context.Background(), peerCredKey, &PeerCred{
		PID:       os.Getpid(),
		UID:       os.Getuid(),
		GID:       os.Getgid(),
		StartTime: startTime,
	})
}

func TestDefaultSocketPath(t *testing.T) {
	path, err := DefaultSocketPath()
	if err != nil {
		t.Fatalf("DefaultSocketPath: %v", err)
	}
	if !strings.HasSuffix(path, "/.env-guard/env-guard.sock") {
		t.Fatalf("unexpected socket path: %s", path)
	}
}

func TestReadProcStartTime(t *testing.T) {
	st, err := readProcStartTime(os.Getpid())
	if err != nil {
		t.Fatalf("readProcStartTime(self): %v", err)
	}
	if st == 0 {
		t.Fatal("expected non-zero start time")
	}
}

func TestMatchesPath(t *testing.T) {
	tests := []struct {
		exe     string
		allowed []string
		want    bool
	}{
		{"/usr/bin/foo", []string{"/usr/bin/foo"}, true},
		{"/usr/bin/foo", []string{"/usr/bin/bar"}, false},
		{"/usr/bin/foo", []string{}, false},
		{"/usr/bin/foo", []string{"/usr/bin/bar", "/usr/bin/foo"}, true},
	}
	for _, tt := range tests {
		got := matchesPath(tt.exe, tt.allowed)
		if got != tt.want {
			t.Errorf("matchesPath(%q, %v) = %v, want %v", tt.exe, tt.allowed, got, tt.want)
		}
	}
}

func TestHealthHandler(t *testing.T) {
	d := New(&mockVault{}, &mockConfig{apps: map[string][]string{}}, t.TempDir()+"/sock")
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	d.healthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.Status != "running" {
		t.Fatalf("expected status 'running', got %q", resp.Status)
	}
	if resp.PID != os.Getpid() {
		t.Fatalf("expected PID %d, got %d", os.Getpid(), resp.PID)
	}
}

func TestSecretsHandlerMissingApp(t *testing.T) {
	d := New(&mockVault{}, &mockConfig{apps: map[string][]string{}}, t.TempDir()+"/sock")
	req := httptest.NewRequest("GET", "/secrets", nil)
	rec := httptest.NewRecorder()
	d.secretsHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSecretsHandlerNoAuth(t *testing.T) {
	d := New(&mockVault{}, &mockConfig{apps: map[string][]string{}}, t.TempDir()+"/sock")
	req := httptest.NewRequest("GET", "/secrets?app=myapp", nil)
	rec := httptest.NewRecorder()
	d.secretsHandler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestSecretsHandlerAuthSuccess(t *testing.T) {
	mv := &mockVault{
		secrets: map[string]map[string]string{
			"myapp": {"DATABASE_URL": "postgres://localhost", "API_KEY": "secret123"},
		},
	}
	mc := &mockConfig{
		apps: map[string][]string{"myapp": nil},
	}
	d := New(mv, mc, t.TempDir()+"/sock")

	req := httptest.NewRequest("GET", "/secrets?app=myapp", nil).WithContext(testCredCtx(t))
	rec := httptest.NewRecorder()
	d.secretsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp secretsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.App != "myapp" {
		t.Fatalf("expected app 'myapp', got %q", resp.App)
	}
	if len(resp.Secrets) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(resp.Secrets))
	}
	if resp.Secrets["DATABASE_URL"] != "postgres://localhost" {
		t.Fatalf("unexpected DATABASE_URL: %q", resp.Secrets["DATABASE_URL"])
	}
	if resp.Secrets["API_KEY"] != "secret123" {
		t.Fatalf("unexpected API_KEY: %q", resp.Secrets["API_KEY"])
	}
}

func TestSecretsHandlerAppNotFound(t *testing.T) {
	mv := &mockVault{secrets: map[string]map[string]string{}}
	mc := &mockConfig{apps: map[string][]string{}}
	d := New(mv, mc, t.TempDir()+"/sock")

	req := httptest.NewRequest("GET", "/secrets?app=nonexistent", nil).WithContext(testCredCtx(t))
	rec := httptest.NewRecorder()
	d.secretsHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSecretsHandlerLogsAccess(t *testing.T) {
	mv := &mockVault{
		secrets: map[string]map[string]string{
			"myapp": {"KEY": "val"},
		},
	}
	mc := &mockConfig{
		apps: map[string][]string{"myapp": nil},
	}
	d := New(mv, mc, t.TempDir()+"/sock")

	req := httptest.NewRequest("GET", "/secrets?app=myapp", nil).WithContext(testCredCtx(t))
	rec := httptest.NewRecorder()
	d.secretsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !mv.logged {
		t.Fatal("expected LogAccess to be called")
	}
	if mv.logApp != "myapp" {
		t.Fatalf("expected log app 'myapp', got %q", mv.logApp)
	}
	if mv.logPID != os.Getpid() {
		t.Fatalf("expected log PID %d, got %d", os.Getpid(), mv.logPID)
	}
}

func TestSecretsHandlerAllowedPath(t *testing.T) {
	exe, err := os.Readlink("/proc/self/exe")
	if err != nil {
		t.Skipf("cannot read /proc/self/exe: %v", err)
	}

	mv := &mockVault{
		secrets: map[string]map[string]string{
			"myapp": {"KEY": "val"},
		},
	}
	mc := &mockConfig{
		apps: map[string][]string{"myapp": {exe}},
	}
	d := New(mv, mc, t.TempDir()+"/sock")

	req := httptest.NewRequest("GET", "/secrets?app=myapp", nil).WithContext(testCredCtx(t))
	rec := httptest.NewRecorder()
	d.secretsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with allowed path, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSecretsHandlerDeniedPath(t *testing.T) {
	mv := &mockVault{
		secrets: map[string]map[string]string{
			"myapp": {"KEY": "val"},
		},
	}
	mc := &mockConfig{
		apps: map[string][]string{"myapp": {"/usr/bin/unauthorized"}},
	}
	d := New(mv, mc, t.TempDir()+"/sock")

	req := httptest.NewRequest("GET", "/secrets?app=myapp", nil).WithContext(testCredCtx(t))
	rec := httptest.NewRecorder()
	d.secretsHandler(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStartStopDaemon(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	mv := &mockVault{
		secrets: map[string]map[string]string{
			"myapp": {"DATABASE_URL": "postgres://localhost"},
		},
	}
	mc := &mockConfig{
		apps: map[string][]string{"myapp": nil},
	}
	d := New(mv, mc, socketPath)

	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Fatal("socket file not created")
	}

	if err := d.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatal("socket file not removed after stop")
	}
}

func TestIntegrationDaemonOverSocket(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "integration.sock")

	dbPath := filepath.Join(t.TempDir(), "vault.db")
	v := vault.New(dbPath)
	if err := v.Create("test-password"); err != nil {
		t.Fatalf("Create vault: %v", err)
	}
	if err := v.CreateProject("myapp"); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := v.SetSecret("myapp", "DATABASE_URL", "postgres://localhost"); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	if err := v.SetSecret("myapp", "API_KEY", "secret123"); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}

	mc := &mockConfig{
		apps: map[string][]string{"myapp": nil},
	}
	d := New(v, mc, socketPath)
	if err := d.Start(); err != nil {
		t.Fatalf("Daemon Start: %v", err)
	}
	defer d.Stop()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	t.Run("health", func(t *testing.T) {
		resp, err := client.Get("http://unix/health")
		if err != nil {
			t.Fatalf("GET /health: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var h healthResponse
		if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
			t.Fatalf("decoding health: %v", err)
		}
		if h.Status != "running" {
			t.Fatalf("expected status 'running', got %q", h.Status)
		}
	})

	t.Run("secrets", func(t *testing.T) {
		resp, err := client.Get("http://unix/secrets?app=myapp")
		if err != nil {
			t.Fatalf("GET /secrets: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
		}

		var sr secretsResponse
		if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
			t.Fatalf("decoding secrets: %v", err)
		}
		if sr.App != "myapp" {
			t.Fatalf("expected app 'myapp', got %q", sr.App)
		}
		if sr.Secrets["DATABASE_URL"] != "postgres://localhost" {
			t.Fatalf("unexpected DATABASE_URL: %q", sr.Secrets["DATABASE_URL"])
		}
		if sr.Secrets["API_KEY"] != "secret123" {
			t.Fatalf("unexpected API_KEY: %q", sr.Secrets["API_KEY"])
		}
	})

	t.Run("secrets missing app", func(t *testing.T) {
		resp, err := client.Get("http://unix/secrets")
		if err != nil {
			t.Fatalf("GET /secrets (no app): %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("secrets unknown app", func(t *testing.T) {
		resp, err := client.Get("http://unix/secrets?app=nonexistent")
		if err != nil {
			t.Fatalf("GET /secrets?app=nonexistent: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})
}
