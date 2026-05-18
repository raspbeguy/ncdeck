// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/raspbeguy/ncdeck/internal/api"
	"github.com/raspbeguy/ncdeck/internal/config"
	"github.com/spf13/cobra"
)

var (
	flagConfig  string
	flagURL     string
	flagJSON    bool
	flagNoColor bool
)

var rootCmd = &cobra.Command{
	Use:          "ncdeck",
	Short:        "Nextcloud Deck CLI + TUI",
	Long:         "ncdeck is a CLI and TUI client for Nextcloud Deck. Use it interactively or from scripts (`--json`).",
	Version:      "dev",
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// https://no-color.org convention.
		if flagNoColor || os.Getenv("NO_COLOR") != "" {
			lipgloss.SetColorProfile(termenv.Ascii)
		}
		return nil
	},
}

// Set by main from GoReleaser-injected ldflags; flows into cobra's --version.
func SetVersion(v, commit, date string) {
	rootCmd.Version = fmt.Sprintf("%s (commit %s, built %s)", v, commit, date)
}

func Execute() error {
	return rootCmd.ExecuteContext(context.Background())
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "path to config file (default ~/.config/ncdeck/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&flagURL, "url", "", "Nextcloud base URL (overrides config)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "output JSON instead of a table")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "disable ANSI color output (also honours NO_COLOR)")
}

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
