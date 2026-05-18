// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Password holds a Nextcloud app password (not the user's real password).
// TODO(keyring): move into OS keychain (github.com/zalando/go-keyring) with
// a plaintext fallback for headless platforms.
type Config struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func DefaultPath() string {
	if v := os.Getenv("NCDECK_CONFIG"); v != "" {
		return v
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "ncdeck", "config.yaml")
}

// Env vars override file values so env-only configuration works for CI/agents.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	if path == "" {
		path = DefaultPath()
	}
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if v := os.Getenv("NCDECK_URL"); v != "" {
		cfg.URL = v
	}
	if v := os.Getenv("NCDECK_USER"); v != "" {
		cfg.Username = v
	}
	if v := os.Getenv("NCDECK_TOKEN"); v != "" {
		cfg.Password = v
	}
	return cfg, nil
}

func Save(path string, cfg *Config) error {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (c *Config) Validate() error {
	switch {
	case c.URL == "":
		return fmt.Errorf("no Nextcloud URL configured (run `ncdeck login` or `ncdeck config set`)")
	case c.Username == "":
		return fmt.Errorf("no username configured")
	case c.Password == "":
		return fmt.Errorf("no app password configured")
	}
	return nil
}
