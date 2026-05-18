// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"fmt"
	"strconv"

	"github.com/raspbeguy/ncdeck/internal/output"
	"github.com/spf13/cobra"
)

var commentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Read and post comments on cards",
}

var (
	commentLimit  int
	commentOffset int
)

var commentListCmd = &cobra.Command{
	Use:   "ls <cardID>",
	Aliases: []string{"list"},
	Short: "List comments on a card",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cardID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid card id %q", args[0])
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		comments, err := c.ListComments(cmd.Context(), cardID, commentLimit, commentOffset)
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), comments)
		}
		rows := make([][]string, 0, len(comments))
		for _, m := range comments {
			rows = append(rows, []string{strconv.Itoa(m.ID), m.ActorDisplay, m.CreationDT.Format("2006-01-02 15:04"), truncate(m.Message, 80)})
		}
		output.Table(cmd.OutOrStdout(), []string{"ID", "AUTHOR", "WHEN", "MESSAGE"}, rows)
		return nil
	},
}

var commentAddCmd = &cobra.Command{
	Use:   "add <cardID> <message>",
	Short: "Post a comment on a card",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cardID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid card id %q", args[0])
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		m, err := c.AddComment(cmd.Context(), cardID, args[1], 0)
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), m)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Posted comment %d\n", m.ID)
		return nil
	},
}

func init() {
	commentListCmd.Flags().IntVar(&commentLimit, "limit", 20, "max comments to return")
	commentListCmd.Flags().IntVar(&commentOffset, "offset", 0, "skip the first N comments")
	commentCmd.AddCommand(commentListCmd, commentAddCmd)
	rootCmd.AddCommand(commentCmd)
}
