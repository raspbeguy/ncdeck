// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the on-disk configuration. The Password field holds a Nextcloud
// app password (not the user's real password) and is currently persisted as
// plaintext in a 0600-mode file under XDG_CONFIG_HOME.
//
// TODO(keyring): move Password into the OS keychain via
// github.com/zalando/go-keyring (or similar), falling back to plaintext on
// platforms where no keyring is available (headless CI, etc.).
type Config struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// DefaultPath returns ~/.config/ncdeck/config.yaml, honoring XDG_CONFIG_HOME.
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

// Load reads the config file (if any) and overlays NCDECK_URL/USER/TOKEN env vars.
// A missing file is not an error, env-only configuration is supported.
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

// Save writes the config to path with mode 0600, creating parent dirs as needed.
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

// Validate returns an error describing what's missing for API calls.
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
