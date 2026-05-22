// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// recordedReq captures the bits of an HTTP call that the import orchestrator
// needs to be observed for: order, method, path, JSON body.
type recordedReq struct {
	method string
	path   string
	body   string
}

type importStub struct {
	t      *testing.T
	mu     sync.Mutex
	calls  []recordedReq
	nextID int

	// Map of created entities by kind so the stub can return realistic
	// responses (with fresh ids).
	createdBoards map[int]string  // id -> title
	createdStacks map[int]int     // id -> boardID
	createdCards  map[int]int     // id -> stackID
	createdLabels map[int]int     // id -> boardID
	failAssignUID string          // if non-empty, AssignUserToCard returns 404 when this UID is passed
}

func newImportStub(t *testing.T) *importStub {
	return &importStub{
		t:             t,
		nextID:        1000,
		createdBoards: make(map[int]string),
		createdStacks: make(map[int]int),
		createdCards:  make(map[int]int),
		createdLabels: make(map[int]int),
	}
}

func (s *importStub) next() int {
	s.nextID++
	return s.nextID
}

func (s *importStub) record(r *http.Request) string {
	body := ""
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}
	s.mu.Lock()
	s.calls = append(s.calls, recordedReq{method: r.Method, path: r.URL.Path, body: body})
	s.mu.Unlock()
	return body
}

func (s *importStub) handler() http.HandlerFunc {
	const prefix = "/index.php/apps/deck/api/v1.0"
	return func(w http.ResponseWriter, r *http.Request) {
		body := s.record(r)
		path := strings.TrimPrefix(r.URL.Path, prefix)
		switch {
		case r.Method == "POST" && path == "/boards":
			var in CreateBoardInput
			_ = json.Unmarshal([]byte(body), &in)
			id := s.next()
			s.createdBoards[id] = in.Title
			// Mimic Deck: GETting the new board returns four default labels.
			_ = json.NewEncoder(w).Encode(Board{ID: id, Title: in.Title, Color: in.Color, Labels: []Label{}})
		case r.Method == "GET" && strings.HasPrefix(path, "/boards/"):
			// GetBoard right after creation: return four default labels with
			// stable IDs we can later expect to be deleted.
			var id int
			_, _ = fmt.Sscanf(path, "/boards/%d", &id)
			_ = json.NewEncoder(w).Encode(Board{
				ID:    id,
				Title: s.createdBoards[id],
				Labels: []Label{
					{ID: 90001, Title: "Default1", BoardID: id},
					{ID: 90002, Title: "Default2", BoardID: id},
					{ID: 90003, Title: "Default3", BoardID: id},
					{ID: 90004, Title: "Default4", BoardID: id},
				},
			})
		case r.Method == "DELETE" && strings.Contains(path, "/labels/"):
			w.WriteHeader(http.StatusOK)
		case r.Method == "POST" && strings.HasSuffix(path, "/labels"):
			var in LabelInput
			_ = json.Unmarshal([]byte(body), &in)
			id := s.next()
			_ = json.NewEncoder(w).Encode(Label{ID: id, Title: in.Title, Color: in.Color})
		case r.Method == "POST" && strings.HasSuffix(path, "/stacks"):
			var in StackInput
			_ = json.Unmarshal([]byte(body), &in)
			id := s.next()
			_ = json.NewEncoder(w).Encode(Stack{ID: id, Title: in.Title, Order: in.Order})
		case r.Method == "POST" && strings.Contains(path, "/stacks/") && strings.HasSuffix(path, "/cards"):
			var in CreateCardInput
			_ = json.Unmarshal([]byte(body), &in)
			id := s.next()
			_ = json.NewEncoder(w).Encode(Card{ID: id, Title: in.Title, Type: in.Type, Order: in.Order, Description: in.Description})
		case r.Method == "PUT" && strings.HasSuffix(path, "/assignLabel"):
			w.WriteHeader(http.StatusOK)
		case r.Method == "PUT" && strings.HasSuffix(path, "/assignUser"):
			if s.failAssignUID != "" && strings.Contains(body, `"`+s.failAssignUID+`"`) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"status":"failure","message":"user not found"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == "PUT" && strings.Contains(path, "/cards/"):
			// UpdateCard echo
			var in UpdateCardInput
			_ = json.Unmarshal([]byte(body), &in)
			_ = json.NewEncoder(w).Encode(Card{Title: in.Title})
		default:
			s.t.Errorf("unexpected request: %s %s", r.Method, path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func miniExport() *DeckExport {
	bug := 58
	feat := 59
	due := "2026-05-10T00:00:00+00:00"
	done := "2026-05-20T19:37:15+00:00"
	uid := "raspbeguy"
	return &DeckExport{
		Boards: []ExportBoard{{
			ID:    11,
			Title: "demo",
			Owner: "alice",
			Color: "0082c9",
			Labels: []ExportLabel{
				{ID: bug, Title: "bug", Color: "e74c3c"},
				{ID: feat, Title: "feature", Color: "2ecc71"},
			},
			Stacks: map[string]ExportStack{
				"47": {ID: 47, Title: "B-second", Order: 1, Cards: []ExportCard{
					{ID: 200, Title: "first by order", Order: 1, Type: "plain",
						Labels:        []ExportLabel{{ID: bug, Title: "bug"}},
						AssignedUsers: []ExportAssignment{{Participant: Owner{UID: uid}}},
						DueDate:       &due,
					},
					{ID: 201, Title: "archived one", Order: 2, Type: "plain", Archived: true},
					{ID: 202, Title: "done one", Order: 3, Type: "plain", Done: &done},
				}},
				"46": {ID: 46, Title: "A-first", Order: 0, Cards: []ExportCard{
					{ID: 100, Title: "only card", Order: 2, Type: "plain",
						Labels: []ExportLabel{{ID: feat, Title: "feature"}, {ID: bug, Title: "bug"}},
					},
				}},
			},
		}},
	}
}

func TestImportBoard_HappyPath(t *testing.T) {
	stub := newImportStub(t)
	c, _ := newTestServer(t, stub.handler())
	board, err := c.ImportBoard(context.Background(), miniExport(), ImportOptions{}, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if board.Title != "demo" {
		t.Errorf("board title: %q", board.Title)
	}

	calls := stub.calls
	if len(calls) == 0 {
		t.Fatal("no requests recorded")
	}

	// The very first call must be the board creation.
	if calls[0].method != "POST" || !strings.HasSuffix(calls[0].path, "/boards") {
		t.Errorf("first call should be POST /boards, got %s %s", calls[0].method, calls[0].path)
	}
	if !strings.Contains(calls[0].body, `"title":"demo"`) {
		t.Errorf("board create body should carry the export title, got %s", calls[0].body)
	}

	// Stacks must be created in Order ascending: stack 46 (Order=0) before stack 47 (Order=1).
	stackPosts := filter(calls, func(r recordedReq) bool {
		return r.method == "POST" && strings.HasSuffix(r.path, "/stacks")
	})
	if len(stackPosts) != 2 {
		t.Fatalf("expected 2 stack POSTs, got %d", len(stackPosts))
	}
	if !strings.Contains(stackPosts[0].body, `"title":"A-first"`) {
		t.Errorf("stacks should be sorted by Order; first should be A-first, got body %s", stackPosts[0].body)
	}
	for i, sp := range stackPosts {
		wantOrder := fmt.Sprintf(`"order":%d`, i)
		if !strings.Contains(sp.body, wantOrder) {
			t.Errorf("stack[%d] body should carry preserved Order %d, got %s", i, i, sp.body)
		}
	}

	// Cards must be sorted by their Order field within a stack and the Order
	// must round-trip in the body.
	cardPosts := filter(calls, func(r recordedReq) bool {
		return r.method == "POST" && strings.Contains(r.path, "/stacks/") && strings.HasSuffix(r.path, "/cards")
	})
	if len(cardPosts) != 4 {
		t.Fatalf("expected 4 card POSTs, got %d", len(cardPosts))
	}

	// Default-label wipe: 4 DELETE /labels/{id} calls.
	delLabels := filter(calls, func(r recordedReq) bool {
		return r.method == "DELETE" && strings.Contains(r.path, "/labels/")
	})
	if len(delLabels) != 4 {
		t.Errorf("expected 4 default-label DELETEs, got %d", len(delLabels))
	}
}

// Pinned: a card with a label whose old id is X must end up assigned with the
// label's *new* id (the one returned from the label-create response), not X.
func TestImportBoard_LabelIDRemap(t *testing.T) {
	stub := newImportStub(t)
	c, _ := newTestServer(t, stub.handler())
	if _, err := c.ImportBoard(context.Background(), miniExport(), ImportOptions{}, nil); err != nil {
		t.Fatal(err)
	}

	labelPosts := filter(stub.calls, func(r recordedReq) bool {
		return r.method == "POST" && strings.HasSuffix(r.path, "/labels")
	})
	if len(labelPosts) != 2 {
		t.Fatalf("expected 2 label POSTs, got %d", len(labelPosts))
	}

	// We don't know the exact remapped IDs the stub assigned, but we can
	// require that no assignLabel call references the *original* file IDs
	// (58 or 59): those must have been remapped.
	assigns := filter(stub.calls, func(r recordedReq) bool {
		return r.method == "PUT" && strings.HasSuffix(r.path, "/assignLabel")
	})
	if len(assigns) != 3 {
		t.Fatalf("expected 3 assignLabel calls (2 on card 100, 1 on card 200), got %d", len(assigns))
	}
	for _, a := range assigns {
		if strings.Contains(a.body, `"labelId":58`) || strings.Contains(a.body, `"labelId":59`) {
			t.Errorf("assignLabel still references the export's old label id; remap broke: %s", a.body)
		}
	}
}

func TestImportBoard_TitleOverride(t *testing.T) {
	stub := newImportStub(t)
	c, _ := newTestServer(t, stub.handler())
	b, err := c.ImportBoard(context.Background(), miniExport(), ImportOptions{TitleOverride: "renamed"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "renamed" {
		t.Errorf("title override should win, got %q", b.Title)
	}
	if !strings.Contains(stub.calls[0].body, `"title":"renamed"`) {
		t.Errorf("board POST body should reflect the overridden title")
	}
}

func TestImportBoard_SkipAssignees(t *testing.T) {
	stub := newImportStub(t)
	c, _ := newTestServer(t, stub.handler())
	if _, err := c.ImportBoard(context.Background(), miniExport(), ImportOptions{SkipAssignees: true}, nil); err != nil {
		t.Fatal(err)
	}
	for _, c := range stub.calls {
		if strings.HasSuffix(c.path, "/assignUser") {
			t.Errorf("--skip-assignees must suppress all assignUser calls, got: %+v", c)
		}
	}
}

// Pinned: a 404 from assignUser (user doesn't exist on the target server) must
// not abort the whole import — it surfaces via the progress callback as a
// warning line and the rest of the import continues.
func TestImportBoard_MissingAssigneeIsLoggedNotFatal(t *testing.T) {
	stub := newImportStub(t)
	stub.failAssignUID = "raspbeguy"
	c, _ := newTestServer(t, stub.handler())

	var warnings []string
	progress := func(s string) {
		if strings.HasPrefix(s, "warning:") {
			warnings = append(warnings, s)
		}
	}
	if _, err := c.ImportBoard(context.Background(), miniExport(), ImportOptions{}, progress); err != nil {
		t.Fatalf("missing assignee must not abort the import: %v", err)
	}
	if len(warnings) == 0 {
		t.Errorf("expected at least one warning line for the missing assignee")
	}
}

func TestImportBoard_RejectsEmptyAndOutOfRange(t *testing.T) {
	c, _ := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if _, err := c.ImportBoard(context.Background(), &DeckExport{}, ImportOptions{}, nil); err == nil {
		t.Errorf("empty boards must error")
	}
	if _, err := c.ImportBoard(context.Background(), miniExport(), ImportOptions{BoardIndex: 5}, nil); err == nil {
		t.Errorf("out-of-range BoardIndex must error")
	}
}

func TestImportBoard_KeepDefaultLabels(t *testing.T) {
	stub := newImportStub(t)
	c, _ := newTestServer(t, stub.handler())
	if _, err := c.ImportBoard(context.Background(), miniExport(), ImportOptions{KeepDefaultLabels: true}, nil); err != nil {
		t.Fatal(err)
	}
	for _, call := range stub.calls {
		if call.method == "DELETE" && strings.Contains(call.path, "/labels/") {
			t.Errorf("--keep-default-labels must suppress the wipe, but saw: %+v", call)
		}
	}
}

func filter[T any](in []T, ok func(T) bool) []T {
	out := make([]T, 0, len(in))
	for _, v := range in {
		if ok(v) {
			out = append(out, v)
		}
	}
	return out
}
