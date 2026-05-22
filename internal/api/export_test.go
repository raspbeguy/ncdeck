// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func exportFixtureHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/index.php/apps/deck/api/v1.0/boards/11":
			_ = json.NewEncoder(w).Encode(Board{
				ID:           11,
				Title:        "demo",
				OwnerRaw:     Owner{UID: "alice", PrimaryKey: "alice", DisplayName: "alice"},
				Color:        "0082c9",
				Archived:     false,
				Labels:       []Label{{ID: 58, Title: "bug", Color: "e74c3c", BoardID: 11}},
				LastModified: 1779305832,
				ETag:         "etag-board",
			})
		case r.URL.Path == "/index.php/apps/deck/api/v1.0/boards/11/stacks":
			_ = json.NewEncoder(w).Encode([]Stack{
				{
					ID: 46, Title: "Backlog", BoardID: 11, Order: 0,
					LastModified: 1779305832, ETag: "etag-stack",
					Cards: []Card{
						{
							ID: 123, Title: "Migrate auth", StackID: 46, Type: "plain",
							Owner:        Owner{UID: "alice", PrimaryKey: "alice", DisplayName: "alice"},
							Labels:       []Label{{ID: 58, Title: "bug", Color: "e74c3c", BoardID: 11}},
							Order:        2,
							LastModified: 1779305831, CreatedAt: 1779305830,
							ETag: "etag-c123",
						},
						{
							ID: 124, Title: "First card", StackID: 46, Type: "plain",
							Owner:        Owner{UID: "alice", PrimaryKey: "alice", DisplayName: "alice"},
							Order:        1,
							LastModified: 1779305832, CreatedAt: 1779305831,
							ETag: "etag-c124",
						},
					},
				},
			})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestExportBoard_AssemblesOccCompatibleShape(t *testing.T) {
	c, _ := newTestServer(t, exportFixtureHandler(t))
	export, err := c.ExportBoard(context.Background(), 11)
	if err != nil {
		t.Fatal(err)
	}

	if len(export.Boards) != 1 {
		t.Fatalf("expected exactly one board in the export wrapper, got %d", len(export.Boards))
	}
	b := export.Boards[0]
	if b.ID != 11 || b.Title != "demo" {
		t.Errorf("board metadata: id=%d title=%q", b.ID, b.Title)
	}
	if b.Owner != "alice" {
		t.Errorf("board.owner must be the bare UID string (occ schema), got %q", b.Owner)
	}
	if _, ok := b.Stacks["46"]; !ok {
		t.Fatalf("stacks must be a map keyed by stringified stack id, got keys=%v", keysOf(b.Stacks))
	}
	if len(b.Labels) != 1 || b.Labels[0].CardID != nil {
		t.Errorf("board-level labels must have cardId=null, got %+v", b.Labels)
	}

	stack := b.Stacks["46"]
	if len(stack.Cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(stack.Cards))
	}
	if stack.Cards[0].ID != 124 || stack.Cards[1].ID != 123 {
		t.Errorf("cards must be sorted by Order ascending; got ids %d,%d", stack.Cards[0].ID, stack.Cards[1].ID)
	}

	c123 := stack.Cards[1]
	if len(c123.Labels) != 1 || c123.Labels[0].CardID == nil || *c123.Labels[0].CardID != 123 {
		t.Errorf("card label cardId pointer must be set to the parent card's id, got %+v", c123.Labels)
	}

	// Golden-shape checks: marshal and verify the schema's distinguishing fields
	// land at the right places (occ has very specific naming, e.g. "duedate" not "dueDate").
	out, err := json.Marshal(export)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		`"owner":"alice"`,
		`"stacks":{"46":`,
		`"duedate":null`,
		`"attachments":null`,
		`"attachmentCount":null`,
		`"relatedStack":null`,
		`"relatedBoard":null`,
		`"ETag":"etag-board"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("expected JSON to contain %q\nfull: %s", want, s)
		}
	}
	for _, unwanted := range []string{
		`"dueDate"`, // wrong-case alternative that would silently break compatibility
		`"comments"`, // occ deck:export omits comments entirely
	} {
		if strings.Contains(s, unwanted) {
			t.Errorf("JSON must not contain %q (breaks occ compatibility)\nfull: %s", unwanted, s)
		}
	}
}

func keysOf(m map[string]ExportStack) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
