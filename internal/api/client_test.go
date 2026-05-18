// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New(srv.URL, "alice", "secret")
	return c, srv
}

func TestListBoards(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/index.php/apps/deck/api/v1.0/boards" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if u, p, _ := r.BasicAuth(); u != "alice" || p != "secret" {
			t.Errorf("bad auth: %s/%s", u, p)
		}
		if r.Header.Get("OCS-APIRequest") != "true" {
			t.Errorf("missing OCS-APIRequest header")
		}
		_ = json.NewEncoder(w).Encode([]Board{
			{ID: 1, Title: "Demo", Color: "0082c9"},
			{ID: 2, Title: "Other", Color: "ff7f50"},
		})
	})
	boards, err := c.ListBoards(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(boards) != 2 || boards[0].Title != "Demo" {
		t.Errorf("unexpected boards: %+v", boards)
	}
}

func TestCreateBoard(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var in CreateBoardInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Fatal(err)
		}
		if in.Title != "My Board" || in.Color != "abcdef" {
			t.Errorf("unexpected body: %+v", in)
		}
		_ = json.NewEncoder(w).Encode(Board{ID: 42, Title: in.Title, Color: in.Color})
	})
	b, err := c.CreateBoard(context.Background(), CreateBoardInput{Title: "My Board", Color: "abcdef"})
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != 42 {
		t.Errorf("expected ID 42, got %d", b.ID)
	}
}

func TestAPIError(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"status":"failure","message":"forbidden"}`))
	})
	_, err := c.GetBoard(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if ae.Status != 403 || ae.Message != "forbidden" {
		t.Errorf("unexpected APIError: %+v", ae)
	}
}

func TestCreateCardDefaultsType(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if got := body["type"]; got != "plain" {
			t.Errorf("expected type=plain, got %v", got)
		}
		_ = json.NewEncoder(w).Encode(Card{ID: 9, Title: "Hi"})
	})
	card, err := c.CreateCard(context.Background(), 1, 1, CreateCardInput{Title: "Hi", Order: 1})
	if err != nil {
		t.Fatal(err)
	}
	if card.ID != 9 {
		t.Errorf("unexpected id: %d", card.ID)
	}
}

// TestReorderCardUsesDestinationStackInPath guards against the bug we hit:
// passing the source stack in the URL path made the server silently no-op the
// stack change.
func TestReorderCardUsesDestinationStackInPath(t *testing.T) {
	var captured string
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.Path
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
	})
	if err := c.ReorderCard(context.Background(), 10, 100, ReorderInput{Order: 3, StackID: 42}); err != nil {
		t.Fatal(err)
	}
	want := "/index.php/apps/deck/api/v1.0/boards/10/stacks/42/cards/100/reorder"
	if captured != want {
		t.Errorf("path: got %q, want %q", captured, want)
	}
}

// TestUpdateCardClearsDueDate verifies that nil DueDate serialises as JSON
// null so callers can actually clear the field.
func TestUpdateCardClearsDueDate(t *testing.T) {
	var payload map[string]any
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(Card{ID: 1})
	})
	_, err := c.UpdateCard(context.Background(), 1, 1, 1, UpdateCardInput{Title: "x"})
	if err != nil {
		t.Fatal(err)
	}
	v, ok := payload["duedate"]
	if !ok {
		t.Fatal("duedate field missing from payload")
	}
	if v != nil {
		t.Errorf("expected JSON null, got %v (%T)", v, v)
	}
	if _, ok := payload["done"]; !ok {
		t.Errorf("done field should be present (as null) for symmetric clear semantics")
	}
}

// TestUploadAttachmentStreams confirms the multipart payload is well-formed
// and that the body is actually streamable (large files).
func TestUploadAttachmentStreams(t *testing.T) {
	tmp, err := os.CreateTemp("", "ncdeck-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString("hello world"); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("bad content-type: %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("OCS-APIRequest") != "true" {
			t.Errorf("missing OCS-APIRequest header")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("type"); got != "file" {
			t.Errorf("type field: %q", got)
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		body, _ := io.ReadAll(f)
		if string(body) != "hello world" {
			t.Errorf("file body: %q", body)
		}
		if !strings.HasPrefix(hdr.Filename, "ncdeck-test-") {
			t.Errorf("filename: %s", hdr.Filename)
		}
		_ = json.NewEncoder(w).Encode(Attachment{ID: 5, Data: hdr.Filename})
	})
	a, err := c.UploadAttachment(context.Background(), 42, tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != 5 {
		t.Errorf("id: %d", a.ID)
	}
}

func TestListCommentsParses(t *testing.T) {
	payload := `{"ocs":{"meta":{"status":"ok","statuscode":200,"message":"OK"},"data":[
		{"id":1,"objectId":42,"message":"hi","actorId":"alice","actorDisplayName":"Alice","actorType":"users","creationDateTime":"2026-01-15T10:00:00+00:00","mentions":[]}
	]}}`
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/ocs/v2.php/apps/deck/api/v1.0/cards/42/comments") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(payload))
	})
	out, err := c.ListComments(context.Background(), 42, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Message != "hi" || out[0].ActorDisplay != "Alice" {
		t.Errorf("unexpected: %+v", out)
	}
}
