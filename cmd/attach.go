// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/raspbeguy/ncdeck/internal/output"
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Manage attachments on cards",
}

var attachDownloadOut string

var attachListCmd = &cobra.Command{
	Use:     "ls <boardID> <stackID> <cardID>",
	Aliases: []string{"list"},
	Short:   "List attachments on a card",
	Args:    cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, stackID, cardID, err := parseTripleID(args)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		a, err := c.ListAttachments(cmd.Context(), boardID, stackID, cardID)
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), a)
		}
		rows := make([][]string, 0, len(a))
		for _, x := range a {
			rows = append(rows, []string{strconv.Itoa(x.ID), x.Data, x.CreatedBy})
		}
		output.Table(cmd.OutOrStdout(), []string{"ID", "FILE", "CREATED BY"}, rows)
		return nil
	},
}

var attachUploadCmd = &cobra.Command{
	Use:   "upload <boardID> <stackID> <cardID> <file>",
	Short: "Upload a local file as a card attachment",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, stackID, cardID, err := parseTripleID(args[:3])
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		a, err := c.UploadAttachment(cmd.Context(), boardID, stackID, cardID, args[3])
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), a)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Uploaded %s as attachment %d\n", args[3], a.ID)
		return nil
	},
}

var attachDownloadCmd = &cobra.Command{
	Use:   "download <boardID> <stackID> <cardID> <attachmentID>",
	Short: "Download an attachment",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, stackID, cardID, err := parseTripleID(args[:3])
		if err != nil {
			return err
		}
		attID, err := strconv.Atoi(args[3])
		if err != nil {
			return fmt.Errorf("invalid attachment id %q", args[3])
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		dst := os.Stdout
		if attachDownloadOut != "" && attachDownloadOut != "-" {
			f, err := os.Create(attachDownloadOut)
			if err != nil {
				return err
			}
			defer f.Close()
			dst = f
		}
		return c.DownloadAttachment(cmd.Context(), boardID, stackID, cardID, attID, dst)
	},
}

func init() {
	attachDownloadCmd.Flags().StringVarP(&attachDownloadOut, "output", "o", "", "destination path (default stdout)")
	attachCmd.AddCommand(attachListCmd, attachUploadCmd, attachDownloadCmd)
	rootCmd.AddCommand(attachCmd)
}
