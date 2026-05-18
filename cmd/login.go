// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/raspbeguy/ncdeck/internal/auth"
	"github.com/raspbeguy/ncdeck/internal/config"
	"github.com/spf13/cobra"
)

var loginURL string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Interactively log in via Nextcloud Login Flow v2",
	RunE: func(cmd *cobra.Command, args []string) error {
		url := strings.TrimSpace(loginURL)
		if url == "" {
			cfg, _ := loadConfig()
			if cfg != nil {
				url = cfg.URL
			}
		}
		if url == "" {
			return fmt.Errorf("--url is required when no URL is configured")
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()

		init, err := auth.Start(ctx, url)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Open this URL in your browser to authorize ncdeck:\n  %s\n\nWaiting for confirmation...\n", init.Login)
		_ = auth.OpenBrowser(init.Login)

		res, err := auth.Poll(ctx, init.Poll)
		if err != nil {
			return err
		}

		cfg, err := config.Load(flagConfig)
		if err != nil {
			return err
		}
		cfg.URL = strings.TrimRight(res.Server, "/")
		cfg.Username = res.LoginName
		cfg.Password = res.AppPassword
		if err := config.Save(flagConfig, cfg); err != nil {
			return err
		}
		dest := flagConfig
		if dest == "" {
			dest = config.DefaultPath()
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s — credentials saved to %s\n", res.LoginName, dest)
		return nil
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginURL, "url", "", "Nextcloud base URL (e.g. https://cloud.example.com)")
	rootCmd.AddCommand(loginCmd)
}
