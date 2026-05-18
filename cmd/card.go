// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/raspbeguy/ncdeck/internal/api"
	"github.com/raspbeguy/ncdeck/internal/output"
	"github.com/spf13/cobra"
)

var cardCmd = &cobra.Command{
	Use:   "card",
	Short: "Manage cards (tasks) within stacks",
}

var (
	cardListStack        int
	cardListArchived     bool
	cardCreateDesc       string
	cardCreateDescFile   string
	cardCreateDue        string
	cardCreateOrder      int
	cardEditTitle        string
	cardEditDesc         string
	cardEditDescFile     string
	cardEditDue          string
	cardEditDone         bool
	cardMoveStack        int
	cardMoveOrder        int
	cardDeleteYes        bool
)

func loadDescription(literal, file string) (string, error) {
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read description file: %w", err)
		}
		return string(b), nil
	}
	return literal, nil
}

func parseDue(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	// Accept either an ISO-8601 timestamp or YYYY-MM-DD (midnight UTC).
	if _, err := time.Parse(time.RFC3339, s); err == nil {
		return s, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return "", fmt.Errorf("invalid date %q (use YYYY-MM-DD or RFC3339)", s)
	}
	return t.Format(time.RFC3339), nil
}

var cardListCmd = &cobra.Command{
	Use:   "ls <boardID>",
	Aliases: []string{"list"},
	Short: "List cards in a board (optionally a single stack)",
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
		var cards []api.Card
		for _, s := range stacks {
			if cardListStack != 0 && s.ID != cardListStack {
				continue
			}
			for _, card := range s.Cards {
				if !cardListArchived && card.Archived {
					continue
				}
				cards = append(cards, card)
			}
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), cards)
		}
		rows := make([][]string, 0, len(cards))
		for _, k := range cards {
			due := ""
			if k.DueDate != nil {
				due = *k.DueDate
			}
			rows = append(rows, []string{
				strconv.Itoa(k.ID),
				strconv.Itoa(k.StackID),
				truncate(k.Title, 60),
				due,
				strconv.Itoa(len(k.Labels)),
				strconv.Itoa(len(k.AssignedUsers)),
			})
		}
		output.Table(cmd.OutOrStdout(), []string{"ID", "STACK", "TITLE", "DUE", "LABELS", "ASSIGNEES"}, rows)
		return nil
	},
}

var cardCreateCmd = &cobra.Command{
	Use:   "create <boardID> <stackID> <title>",
	Short: "Create a card",
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
		desc, err := loadDescription(cardCreateDesc, cardCreateDescFile)
		if err != nil {
			return err
		}
		due, err := parseDue(cardCreateDue)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		k, err := c.CreateCard(cmd.Context(), boardID, stackID, api.CreateCardInput{
			Title:       args[2],
			Description: desc,
			Order:       cardCreateOrder,
			DueDate:     due,
		})
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), k)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Created card %d: %s\n", k.ID, k.Title)
		return nil
	},
}

var cardShowCmd = &cobra.Command{
	Use:   "show <boardID> <stackID> <cardID>",
	Short: "Show a card's details",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, stackID, cardID, err := parseTripleID(args)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		k, err := c.GetCard(cmd.Context(), boardID, stackID, cardID)
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), k)
		}
		due := "(none)"
		if k.DueDate != nil {
			due = *k.DueDate
		}
		fmt.Fprintf(cmd.OutOrStdout(),
			"Card #%d: %s\n  stack:    %d\n  archived: %v\n  due:      %s\n  labels:   %d\n  assignees:%d\n\n%s\n",
			k.ID, k.Title, k.StackID, k.Archived, due, len(k.Labels), len(k.AssignedUsers), k.Description)
		return nil
	},
}

var cardEditCmd = &cobra.Command{
	Use:   "edit <boardID> <stackID> <cardID>",
	Short: "Edit a card's fields",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, stackID, cardID, err := parseTripleID(args)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		cur, err := c.GetCard(cmd.Context(), boardID, stackID, cardID)
		if err != nil {
			return err
		}
		in := api.UpdateCardInput{
			Title:       cur.Title,
			Description: cur.Description,
			Type:        cur.Type,
			Owner:       cur.Owner.UID,
			Order:       cur.Order,
			Archived:    cur.Archived,
		}
		if cur.DueDate != nil {
			in.DueDate = *cur.DueDate
		}
		if cmd.Flags().Changed("title") {
			in.Title = cardEditTitle
		}
		if cmd.Flags().Changed("description") || cmd.Flags().Changed("description-file") {
			d, err := loadDescription(cardEditDesc, cardEditDescFile)
			if err != nil {
				return err
			}
			in.Description = d
		}
		if cmd.Flags().Changed("due") {
			d, err := parseDue(cardEditDue)
			if err != nil {
				return err
			}
			in.DueDate = d
		}
		if cmd.Flags().Changed("done") {
			if cardEditDone {
				in.Done = time.Now().UTC().Format(time.RFC3339)
			} else {
				in.Done = ""
			}
		}
		k, err := c.UpdateCard(cmd.Context(), boardID, stackID, cardID, in)
		if err != nil {
			return err
		}
		if flagJSON {
			return output.JSON(cmd.OutOrStdout(), k)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Updated card %d\n", k.ID)
		return nil
	},
}

var cardMoveCmd = &cobra.Command{
	Use:   "move <boardID> <stackID> <cardID>",
	Short: "Move a card to a different stack and/or position",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, _, cardID, err := parseTripleID(args)
		if err != nil {
			return err
		}
		if cardMoveStack == 0 {
			return fmt.Errorf("--to-stack is required")
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.ReorderCard(cmd.Context(), boardID, cardID, api.ReorderInput{Order: cardMoveOrder, StackID: cardMoveStack}); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Moved card %d to stack %d (order %d)\n", cardID, cardMoveStack, cardMoveOrder)
		return nil
	},
}

var cardAssignCmd = &cobra.Command{
	Use:   "assign <boardID> <stackID> <cardID> <userID>",
	Short: "Assign a user to a card",
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
		if err := c.AssignUserToCard(cmd.Context(), boardID, stackID, cardID, args[3]); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Assigned %s to card %d\n", args[3], cardID)
		return nil
	},
}

var cardUnassignCmd = &cobra.Command{
	Use:   "unassign <boardID> <stackID> <cardID> <userID>",
	Short: "Remove a user assignment",
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
		if err := c.UnassignUserFromCard(cmd.Context(), boardID, stackID, cardID, args[3]); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Unassigned %s from card %d\n", args[3], cardID)
		return nil
	},
}

var cardLabelCmd = &cobra.Command{
	Use:   "label <boardID> <stackID> <cardID> <labelID>",
	Short: "Add a label to a card",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, stackID, cardID, err := parseTripleID(args[:3])
		if err != nil {
			return err
		}
		labelID, err := strconv.Atoi(args[3])
		if err != nil {
			return fmt.Errorf("invalid label id %q", args[3])
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.AssignLabelToCard(cmd.Context(), boardID, stackID, cardID, labelID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added label %d to card %d\n", labelID, cardID)
		return nil
	},
}

var cardUnlabelCmd = &cobra.Command{
	Use:   "unlabel <boardID> <stackID> <cardID> <labelID>",
	Short: "Remove a label from a card",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, stackID, cardID, err := parseTripleID(args[:3])
		if err != nil {
			return err
		}
		labelID, err := strconv.Atoi(args[3])
		if err != nil {
			return fmt.Errorf("invalid label id %q", args[3])
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.RemoveLabelFromCard(cmd.Context(), boardID, stackID, cardID, labelID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removed label %d from card %d\n", labelID, cardID)
		return nil
	},
}

var cardArchiveCmd = &cobra.Command{
	Use:   "archive <boardID> <stackID> <cardID>",
	Short: "Archive a card",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, stackID, cardID, err := parseTripleID(args)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.ArchiveCard(cmd.Context(), boardID, stackID, cardID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Archived card %d\n", cardID)
		return nil
	},
}

var cardUnarchiveCmd = &cobra.Command{
	Use:   "unarchive <boardID> <stackID> <cardID>",
	Short: "Unarchive a card",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, stackID, cardID, err := parseTripleID(args)
		if err != nil {
			return err
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.UnarchiveCard(cmd.Context(), boardID, stackID, cardID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Unarchived card %d\n", cardID)
		return nil
	},
}

var cardDeleteCmd = &cobra.Command{
	Use:   "delete <boardID> <stackID> <cardID>",
	Short: "Delete a card",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		boardID, stackID, cardID, err := parseTripleID(args)
		if err != nil {
			return err
		}
		if !cardDeleteYes && !confirm(cmd, fmt.Sprintf("Delete card %d?", cardID)) {
			return fmt.Errorf("aborted")
		}
		c, err := newClient()
		if err != nil {
			return err
		}
		if err := c.DeleteCard(cmd.Context(), boardID, stackID, cardID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted card %d\n", cardID)
		return nil
	},
}

// parseTripleID parses three positional args into board/stack/card IDs.
func parseTripleID(args []string) (int, int, int, error) {
	if len(args) < 3 {
		return 0, 0, 0, fmt.Errorf("expected boardID stackID cardID")
	}
	bid, err := strconv.Atoi(args[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid board id %q", args[0])
	}
	sid, err := strconv.Atoi(args[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid stack id %q", args[1])
	}
	cid, err := strconv.Atoi(args[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid card id %q", args[2])
	}
	return bid, sid, cid, nil
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func init() {
	cardListCmd.Flags().IntVar(&cardListStack, "stack", 0, "filter by stack ID")
	cardListCmd.Flags().BoolVar(&cardListArchived, "archived", false, "include archived cards")

	cardCreateCmd.Flags().StringVar(&cardCreateDesc, "description", "", "card description (markdown)")
	cardCreateCmd.Flags().StringVarP(&cardCreateDescFile, "description-file", "F", "", "read description from file (- for stdin)")
	cardCreateCmd.Flags().StringVar(&cardCreateDue, "due", "", "due date (YYYY-MM-DD or RFC3339)")
	cardCreateCmd.Flags().IntVar(&cardCreateOrder, "order", 999, "position within stack")

	cardEditCmd.Flags().StringVar(&cardEditTitle, "title", "", "new title")
	cardEditCmd.Flags().StringVar(&cardEditDesc, "description", "", "new description")
	cardEditCmd.Flags().StringVarP(&cardEditDescFile, "description-file", "F", "", "read description from file")
	cardEditCmd.Flags().StringVar(&cardEditDue, "due", "", "new due date (\"\" to clear)")
	cardEditCmd.Flags().BoolVar(&cardEditDone, "done", false, "mark done (true) or undone (false)")

	cardMoveCmd.Flags().IntVar(&cardMoveStack, "to-stack", 0, "destination stack ID")
	cardMoveCmd.Flags().IntVar(&cardMoveOrder, "order", 999, "position within destination stack")

	cardDeleteCmd.Flags().BoolVar(&cardDeleteYes, "yes", false, "skip confirmation")

	cardCmd.AddCommand(
		cardListCmd, cardCreateCmd, cardShowCmd, cardEditCmd, cardMoveCmd,
		cardAssignCmd, cardUnassignCmd, cardLabelCmd, cardUnlabelCmd,
		cardArchiveCmd, cardUnarchiveCmd, cardDeleteCmd,
	)
	rootCmd.AddCommand(cardCmd)
}
