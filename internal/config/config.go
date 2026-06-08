package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Applications map[string]App `yaml:"applications"`
}

type App struct {
	AllowedPaths []string `yaml:"allowed_paths,omitempty"`
	Secrets      []string `yaml:"secrets"`
}

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

func (c *Config) AppNames() []string {
	names := make([]string, 0, len(c.Applications))
	for name := range c.Applications {
		names = append(names, name)
	}
	return names
}
