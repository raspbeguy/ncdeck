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
	Use:   "ls <cardID>",
	Aliases: []string{"list"},
	Short: "List attachments on a card",
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
		a, err := c.ListAttachments(cmd.Context(), cardID)
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
	Use:   "upload <cardID> <file>",
	Short: "Upload a local file as a card attachment",
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
		a, err := c.UploadAttachment(cmd.Context(), cardID, args[1])
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), a)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Uploaded %s as attachment %d\n", args[1], a.ID)
		return nil
	},
}

var attachDownloadCmd = &cobra.Command{
	Use:   "download <cardID> <attachmentID>",
	Short: "Download an attachment",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cardID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid card id %q", args[0])
		}
		attID, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid attachment id %q", args[1])
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
		return c.DownloadAttachment(cmd.Context(), cardID, attID, dst)
	},
}

func init() {
	attachDownloadCmd.Flags().StringVarP(&attachDownloadOut, "output", "o", "", "destination path (default stdout)")
	attachCmd.AddCommand(attachListCmd, attachUploadCmd, attachDownloadCmd)
	rootCmd.AddCommand(attachCmd)
}
