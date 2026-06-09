package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the parsed env-guard.yaml file.
// It maps application names to their secret definitions.
type Config struct {
	Applications map[string]App `yaml:"applications"`
}

// App defines the secrets and optional access restrictions for a single application.
type App struct {
	// AllowedPaths restricts which executables may request this app's secrets
	// via the daemon API. Paths are matched against /proc/<pid>/exe.
	// If empty, any process owned by the user may request the secrets.
	AllowedPaths []string `yaml:"allowed_paths,omitempty"`
	// Secrets is the list of secret keys this app requires (e.g. DATABASE_URL).
	Secrets []string `yaml:"secrets"`
}

// Parse reads and validates a YAML config file.
// Returns an error if the file cannot be read, is invalid YAML,
// or does not define at least one app with at least one secret each.
func Parse(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config yaml: %w", err)
	}

	if len(cfg.Applications) == 0 {
		return nil, fmt.Errorf("config must define at least one app")
	}

	for name, app := range cfg.Applications {
		if len(app.Secrets) == 0 {
			return nil, fmt.Errorf("app %q must define at least one secret", name)
		}
	}

	return &cfg, nil
}

// AppNames returns a sorted list of application names defined in the config.
func (c *Config) AppNames() []string {
	names := make([]string, 0, len(c.Applications))
	for name := range c.Applications {
		names = append(names, name)
	}
	return names
}

// HasApp reports whether the given app name exists in the config.
func (c *Config) HasApp(appName string) bool {
	_, ok := c.Applications[appName]
	return ok
}

// AllowedPaths returns the allowed executable paths for an app, or nil if none.
func (c *Config) AllowedPaths(appName string) []string {
	app, ok := c.Applications[appName]
	if !ok {
		return nil
	}
	return app.AllowedPaths
}
