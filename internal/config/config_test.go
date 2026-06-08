package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "env-guard.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParse_ValidConfig(t *testing.T) {
	yaml := `
applications:
  myapp:
    secrets:
      - DATABASE_URL
      - API_KEY
  otherapp:
    secrets:
      - GITHUB_TOKEN
      - AWS_SECRET_KEY
`
	path := writeTempFile(t, yaml)
	cfg, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Applications) != 2 {
		t.Fatalf("expected 2 app, got %d", len(cfg.Applications))
	}

	myapp, ok := cfg.Applications["myapp"]
	if !ok {
		t.Fatal("expected app 'myapp'")
	}
	if len(myapp.Secrets) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(myapp.Secrets))
	}
	if myapp.Secrets[0] != "DATABASE_URL" {
		t.Fatalf("expected DATABASE_URL, got %s", myapp.Secrets[0])
	}

	appNames := cfg.AppNames()
	if len(appNames) != 2 {
		t.Fatalf("expected 2 app names, got %d", len(appNames))
	}
}

func TestParse_WithAllowedPaths(t *testing.T) {
	yaml := `
applications:
  myapp:
    allowed_paths:
      - /home/user/myapp/bin
      - /home/user/myapp/node_modules/.bin/next
    secrets:
      - DATABASE_URL
`
	path := writeTempFile(t, yaml)
	cfg, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	myapp := cfg.Applications["myapp"]
	if len(myapp.AllowedPaths) != 2 {
		t.Fatalf("expected 2 allowed paths, got %d", len(myapp.AllowedPaths))
	}
	if myapp.AllowedPaths[1] != "/home/user/myapp/node_modules/.bin/next" {
		t.Fatalf("unexpected allowed path: %s", myapp.AllowedPaths[1])
	}
}

func TestParse_MissingFile(t *testing.T) {
	_, err := Parse("/nonexistent/path/env-guard.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParse_EmptySecrets(t *testing.T) {
	yaml := `
applications:
  myapp:
    secrets: []
`
	path := writeTempFile(t, yaml)
	_, err := Parse(path)
	if err == nil {
		t.Fatal("expected error for app with no secrets")
	}
}

func TestParse_Noapp(t *testing.T) {
	yaml := `
applications: {}
`
	path := writeTempFile(t, yaml)
	_, err := Parse(path)
	if err == nil {
		t.Fatal("expected error for empty app")
	}
}

func TestParse_SingleApp(t *testing.T) {
	yaml := `
applications:
  myapp:
    secrets:
      - DATABASE_URL
`
	path := writeTempFile(t, yaml)
	cfg, err := Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Applications) != 1 {
		t.Fatalf("expected 1 app, got %d", len(cfg.Applications))
	}
}
