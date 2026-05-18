// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
