package commands

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/term"

	"github.com/env-guard/env-guard/internal/config"
	"github.com/env-guard/env-guard/internal/daemon"
	"github.com/env-guard/env-guard/internal/vault"
)

func Login() int {
	cfgPath := findConfigYAML()
	if cfgPath == "" {
		fmt.Fprintln(os.Stderr, "No env-guard.yaml found in current directory.")
		return 1
	}

	cfg, err := config.Parse(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid config: %v\n", err)
		return 1
	}

	vaultPath, err := vault.DefaultVaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if _, err := os.Stat(vaultPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "No vault found at %s\n", vaultPath)
		fmt.Fprintln(os.Stderr, "Run env-guard init or launch env-guard (TUI) to create one first.")
		return 1
	}

	var password string

	fmt.Print("Master password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
		return 1
	}
	password = string(passwordBytes)

	if password == "" {
		fmt.Fprintln(os.Stderr, "Password cannot be empty.")
		return 1
	}

	v := vault.New(vaultPath)
	if err := v.Open(password); err != nil {
		if err == vault.ErrWrongPassword {
			fmt.Fprintln(os.Stderr, "Wrong master password.")
		} else {
			fmt.Fprintf(os.Stderr, "Error opening vault: %v\n", err)
		}
		return 1
	}

	socketPath, err := daemon.DefaultSocketPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error determining socket path: %v\n", err)
		v.Close()
		return 1
	}

	d := daemon.New(v, cfg, socketPath)
	if err := d.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting daemon: %v\n", err)
		v.Close()
		return 1
	}

	fmt.Printf("Vault unlocked. Daemon running on %s\n", socketPath)
	fmt.Println("Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down daemon...")
	d.Stop()
	v.Close()
	return 0
}

func findConfigYAML() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	path := filepath.Join(cwd, "env-guard.yaml")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}
