// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/raspbeguy/ncdeck/internal/api"
	"github.com/raspbeguy/ncdeck/internal/config"
	"github.com/spf13/cobra"
)

var (
	flagConfig string
	flagURL    string
	flagJSON   bool
	flagNoColor bool
)

var rootCmd = &cobra.Command{
	Use:     "ncdeck",
	Short:   "Nextcloud Deck CLI + TUI",
	Long:    "ncdeck is a CLI and TUI client for Nextcloud Deck. Use it interactively or from scripts (`--json`).",
	Version: "dev",
	SilenceUsage: true,
}

// SetVersion is called from main with the values injected at build time by
// GoReleaser. The values flow into cobra's built-in --version flag.
func SetVersion(v, commit, date string) {
	rootCmd.Version = fmt.Sprintf("%s (commit %s, built %s)", v, commit, date)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.ExecuteContext(context.Background())
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "path to config file (default ~/.config/ncdeck/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&flagURL, "url", "", "Nextcloud base URL (overrides config)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "output JSON instead of a table")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "disable ANSI color output")
}

// loadConfig honors --config and applies env-var/flag overlays.
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(flagConfig)
	if err != nil {
		return nil, err
	}
	if flagURL != "" {
		cfg.URL = flagURL
	}
	return cfg, nil
}

// newClient returns an authenticated API client or an error if the config is incomplete.
func newClient() (*api.Client, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return api.New(cfg.URL, cfg.Username, cfg.Password), nil
}

// die prints err to stderr and exits non-zero. Used in subcommand RunE returns.
func die(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

// stderr is a convenience for printing to os.Stderr.
func stderr(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}
