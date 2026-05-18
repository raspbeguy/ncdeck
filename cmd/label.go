// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"fmt"
	"strconv"

	"github.com/raspbeguy/ncdeck/internal/api"
	"github.com/raspbeguy/ncdeck/internal/output"
	"github.com/spf13/cobra"
)

var labelCmd = &cobra.Command{
	Use:   "label",
	Short: "Manage board-level labels",
}

var (
	labelCreateColor string
	labelDeleteYes   bool
)

var labelListCmd = &cobra.Command{
	Use:     "ls <boardID>",
	Aliases: []string{"list"},
	Short:   "List labels on a board",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid board id %q", args[0])
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		b, err := c.GetBoard(cmd.Context(), boardID)
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), b.Labels)
		}
		rows := make([][]string, 0, len(b.Labels))
		for _, l := range b.Labels {
			rows = append(rows, []string{strconv.Itoa(l.ID), l.Title, "#" + l.Color})
		}
		output.Table(cmd.OutOrStdout(), []string{"ID", "TITLE", "COLOR"}, rows)
		return nil
	},
}

var labelCreateCmd = &cobra.Command{
	Use:   "create <boardID> <title>",
	Short: "Create a new label",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid board id %q", args[0])
		}
		color := labelCreateColor
		if color == "" {
			color = "888888"
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		l, err := c.CreateLabel(cmd.Context(), boardID, api.LabelInput{Title: args[1], Color: color})
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), l)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Created label %d: %s\n", l.ID, l.Title)
		return nil
	},
}

var labelDeleteCmd = &cobra.Command{
	Use:   "delete <boardID> <labelID>",
	Short: "Delete a label",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid board id %q", args[0])
		}
		labelID, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid label id %q", args[1])
		}
		if !labelDeleteYes && !confirm(cmd, fmt.Sprintf("Delete label %d?", labelID)) {
			return fmt.Errorf("aborted")
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.DeleteLabel(cmd.Context(), boardID, labelID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted label %d\n", labelID)
		return nil
	},
}

func init() {
	labelCreateCmd.Flags().StringVar(&labelCreateColor, "color", "", "hex color (default 888888)")
	labelDeleteCmd.Flags().BoolVar(&labelDeleteYes, "yes", false, "skip confirmation")
	labelCmd.AddCommand(labelListCmd, labelCreateCmd, labelDeleteCmd)
	rootCmd.AddCommand(labelCmd)
}
