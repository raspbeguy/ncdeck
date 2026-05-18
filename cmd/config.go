// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"fmt"

	"github.com/raspbeguy/ncdeck/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage ncdeck configuration",
}

var (
	cfgSetURL   string
	cfgSetUser  string
	cfgSetToken string
)

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set Nextcloud URL, username, and app password",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := flagConfig
		cfg, err := config.Load(path)
		if err != nil {
			return err
		}
		if cfgSetURL != "" {
			cfg.URL = cfgSetURL
		}
		if cfgSetUser != "" {
			cfg.Username = cfgSetUser
		}
		if cfgSetToken != "" {
			cfg.Password = cfgSetToken
		}
		if err := config.Save(path, cfg); err != nil {
			return err
		}
		dest := path
		if dest == "" {
			dest = config.DefaultPath()
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Saved configuration to %s\n", dest)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the current configuration (token redacted)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		redacted := "(unset)"
		if cfg.Password != "" {
			redacted = "(set)"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "url:      %s\nusername: %s\npassword: %s\n", cfg.URL, cfg.Username, redacted)
		return nil
	},
}

func init() {
	configSetCmd.Flags().StringVar(&cfgSetURL, "url", "", "Nextcloud base URL")
	configSetCmd.Flags().StringVar(&cfgSetUser, "user", "", "Nextcloud username")
	configSetCmd.Flags().StringVar(&cfgSetToken, "token", "", "app password / token")
	configCmd.AddCommand(configSetCmd, configShowCmd)
	rootCmd.AddCommand(configCmd)
}
