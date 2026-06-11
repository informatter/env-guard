package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindConfigYAML(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "env-guard.yaml")
		if err := os.WriteFile(path, []byte("applications:\n  test:\n    secrets:\n      - KEY\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		origDir, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(origDir)

		got := findConfigYAML()
		if got != path {
			t.Fatalf("expected %q, got %q", path, got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		dir := t.TempDir()
		origDir, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(origDir)

		got := findConfigYAML()
		if got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})
}

func TestLoginRequiresConfig(t *testing.T) {
	origDir, _ := os.Getwd()
	emptyDir := t.TempDir()
	os.Chdir(emptyDir)
	defer os.Chdir(origDir)

	exitCode := Login()
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 when no config, got %d", exitCode)
	}
}
