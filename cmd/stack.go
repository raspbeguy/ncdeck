// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"fmt"
	"strconv"

	"github.com/raspbeguy/ncdeck/internal/api"
	"github.com/raspbeguy/ncdeck/internal/output"
	"github.com/spf13/cobra"
)

var stackCmd = &cobra.Command{
	Use:   "stack",
	Short: "Manage stacks (columns) within a board",
}

var (
	stackCreateOrder int
	stackDeleteYes   bool
)

var stackListCmd = &cobra.Command{
	Use:   "ls <boardID>",
	Aliases: []string{"list"},
	Short: "List stacks in a board",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid board id %q", args[0])
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		stacks, err := c.ListStacks(cmd.Context(), boardID)
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), stacks)
		}
		rows := make([][]string, 0, len(stacks))
		for _, s := range stacks {
			rows = append(rows, []string{strconv.Itoa(s.ID), s.Title, strconv.Itoa(s.Order), strconv.Itoa(len(s.Cards))})
		}
		output.Table(cmd.OutOrStdout(), []string{"ID", "TITLE", "ORDER", "CARDS"}, rows)
		return nil
	},
}

var stackCreateCmd = &cobra.Command{
	Use:   "create <boardID> <title>",
	Short: "Create a stack",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid board id %q", args[0])
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		s, err := c.CreateStack(cmd.Context(), boardID, api.StackInput{Title: args[1], Order: stackCreateOrder})
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), s)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Created stack %d: %s\n", s.ID, s.Title)
		return nil
	},
}

var stackRenameCmd = &cobra.Command{
	Use:   "rename <boardID> <stackID> <title>",
	Short: "Rename a stack",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid board id %q", args[0])
		}
		stackID, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid stack id %q", args[1])
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		cur, err := c.GetStack(cmd.Context(), boardID, stackID)
		if err != nil {
			return err
		}
		s, err := c.UpdateStack(cmd.Context(), boardID, stackID, api.StackInput{Title: args[2], Order: cur.Order})
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), s)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Renamed stack %d to %s\n", s.ID, s.Title)
		return nil
	},
}

var stackDeleteCmd = &cobra.Command{
	Use:   "delete <boardID> <stackID>",
	Short: "Delete a stack",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid board id %q", args[0])
		}
		stackID, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid stack id %q", args[1])
		}
		if !stackDeleteYes && !confirm(fmt.Sprintf("Delete stack %d?", stackID)) {
			return fmt.Errorf("aborted")
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.DeleteStack(cmd.Context(), boardID, stackID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted stack %d\n", stackID)
		return nil
	},
}

func init() {
	stackCreateCmd.Flags().IntVar(&stackCreateOrder, "order", 999, "position among stacks")
	stackDeleteCmd.Flags().BoolVar(&stackDeleteYes, "yes", false, "skip confirmation")
	stackCmd.AddCommand(stackListCmd, stackCreateCmd, stackRenameCmd, stackDeleteCmd)
	rootCmd.AddCommand(stackCmd)
}
