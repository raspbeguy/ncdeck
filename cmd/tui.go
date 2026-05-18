// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"fmt"
	"strconv"

	"github.com/raspbeguy/ncdeck/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui [boardID]",
	Short: "Launch the interactive TUI",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		boardID := 0
		if len(args) == 1 {
			boardID, err = strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid board id %q", args[0])
			}
		}
		return tui.Run(cmd.Context(), c, boardID)
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
