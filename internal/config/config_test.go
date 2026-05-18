// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPath_RespectsNCDECK_CONFIG(t *testing.T) {
	t.Setenv("NCDECK_CONFIG", "/custom/path.yaml")
	if got := DefaultPath(); got != "/custom/path.yaml" {
		t.Errorf("got %q, want /custom/path.yaml", got)
	}
}

func TestDefaultPath_UsesXDG_CONFIG_HOME(t *testing.T) {
	t.Setenv("NCDECK_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := DefaultPath(); got != "/xdg/ncdeck/config.yaml" {
		t.Errorf("got %q, want /xdg/ncdeck/config.yaml", got)
	}
}

func TestLoad_MissingFileIsNotAnError(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != "" || cfg.Username != "" || cfg.Password != "" {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("url: https://from-file\nusername: file-user\npassword: file-pw\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NCDECK_URL", "https://from-env")
	t.Setenv("NCDECK_TOKEN", "env-pw")
	// Leave NCDECK_USER unset so the file value survives.
	t.Setenv("NCDECK_USER", "")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != "https://from-env" {
		t.Errorf("URL: got %q, want from env", cfg.URL)
	}
	if cfg.Username != "file-user" {
		t.Errorf("Username: got %q, want from file (env was empty)", cfg.Username)
	}
	if cfg.Password != "env-pw" {
		t.Errorf("Password: got %q, want from env", cfg.Password)
	}
}

func TestSave_WritesFileMode0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.yaml")
	if err := Save(path, &Config{URL: "https://x", Username: "u", Password: "p"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode: got %o, want 0600 (config holds an app password)", info.Mode().Perm())
	}
}

func TestValidate_ReportsMissingFields(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"all set", Config{URL: "x", Username: "y", Password: "z"}, false},
		{"missing url", Config{Username: "y", Password: "z"}, true},
		{"missing user", Config{URL: "x", Password: "z"}, true},
		{"missing password", Config{URL: "x", Username: "y"}, true},
	}
	for _, tc := range cases {
		err := tc.cfg.Validate()
		if (err != nil) != tc.want {
			t.Errorf("%s: err=%v wantErr=%v", tc.name, err, tc.want)
		}
	}
}
