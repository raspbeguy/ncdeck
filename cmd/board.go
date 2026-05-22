// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/raspbeguy/ncdeck/internal/api"
	"github.com/raspbeguy/ncdeck/internal/output"
	"github.com/spf13/cobra"
)

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Manage Deck boards",
}

var (
	boardListArchived bool
	boardListDetails  bool
	boardCreateColor  string
	boardUpdateTitle  string
	boardUpdateColor  string
	boardUpdateArch   bool
	boardDeleteYes    bool
	boardExportOut    string
)

var boardListCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List boards",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		boards, err := c.ListBoards(cmd.Context(), boardListDetails)
		if err != nil {
			return err
		}
		if !boardListArchived {
			filtered := boards[:0]
			for _, b := range boards {
				if !b.Archived {
					filtered = append(filtered, b)
				}
			}
			boards = filtered
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), boards)
		}
		rows := make([][]string, 0, len(boards))
		for _, b := range boards {
			archived := ""
			if b.Archived {
				archived = "yes"
			}
			rows = append(rows, []string{strconv.Itoa(b.ID), b.Title, "#" + b.Color, b.OwnerRaw.UID, archived})
		}
		output.Table(cmd.OutOrStdout(), []string{"ID", "TITLE", "COLOR", "OWNER", "ARCHIVED"}, rows)
		return nil
	},
}

var boardCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a board",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		color := boardCreateColor
		if color == "" {
			color = "0082c9"
		}
		b, err := c.CreateBoard(cmd.Context(), api.CreateBoardInput{Title: args[0], Color: color})
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), b)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Created board %d: %s\n", b.ID, b.Title)
		return nil
	},
}

var boardShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a board's details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid board id %q", args[0])
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		b, err := c.GetBoard(cmd.Context(), id)
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), b)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Board #%d: %s\n  color:    #%s\n  owner:    %s\n  archived: %v\n  labels:   %d\n", b.ID, b.Title, b.Color, b.OwnerRaw.UID, b.Archived, len(b.Labels))
		return nil
	},
}

var boardUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a board's title/color/archived flag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid board id %q", args[0])
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		// Fetch current values so unspecified flags are preserved.
		cur, err := c.GetBoard(cmd.Context(), id)
		if err != nil {
			return err
		}
		in := api.UpdateBoardInput{Title: cur.Title, Color: cur.Color, Archived: cur.Archived}
		if cmd.Flags().Changed("title") {
			in.Title = boardUpdateTitle
		}
		if cmd.Flags().Changed("color") {
			in.Color = boardUpdateColor
		}
		if cmd.Flags().Changed("archived") {
			in.Archived = boardUpdateArch
		}
		b, err := c.UpdateBoard(cmd.Context(), id, in)
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), b)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Updated board %d: %s\n", b.ID, b.Title)
		return nil
	},
}

var boardDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a board",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid board id %q", args[0])
		}
		if !boardDeleteYes && !confirm(fmt.Sprintf("Delete board %d?", id)) {
			return fmt.Errorf("aborted")
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.DeleteBoard(cmd.Context(), id); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted board %d\n", id)
		return nil
	},
}

var boardExportCmd = &cobra.Command{
	Use:   "export <boardID>",
	Short: "Export a board to JSON in the occ deck:export schema",
	Long: `Export a board to a JSON file matching the schema produced by Nextcloud's
` + "`occ deck:export`" + ` server-side command. The output can be fed back through
` + "`occ deck:import`" + ` on any Nextcloud instance.

Note: comments and attachment bytes are NOT included; the server's exporter
omits them too.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid board id %q", args[0])
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		export, err := c.ExportBoard(cmd.Context(), id)
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		if boardExportOut != "" {
			f, err := os.Create(boardExportOut)
			if err != nil {
				return err
			}
			defer f.Close()
			out = f
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(export)
	},
}

func init() {
	boardListCmd.Flags().BoolVar(&boardListArchived, "archived", false, "include archived boards")
	boardListCmd.Flags().BoolVar(&boardListDetails, "details", false, "request server-side details")
	boardCreateCmd.Flags().StringVar(&boardCreateColor, "color", "", "hex color without # (default 0082c9)")
	boardUpdateCmd.Flags().StringVar(&boardUpdateTitle, "title", "", "new title")
	boardUpdateCmd.Flags().StringVar(&boardUpdateColor, "color", "", "new color (hex without #)")
	boardUpdateCmd.Flags().BoolVar(&boardUpdateArch, "archived", false, "archive (true) or unarchive (false)")
	boardDeleteCmd.Flags().BoolVar(&boardDeleteYes, "yes", false, "skip confirmation")
	boardExportCmd.Flags().StringVarP(&boardExportOut, "out", "o", "", "output file (default stdout)")

	boardCmd.AddCommand(boardListCmd, boardCreateCmd, boardShowCmd, boardUpdateCmd, boardDeleteCmd, boardExportCmd)
	rootCmd.AddCommand(boardCmd)
}
