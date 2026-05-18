// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestStart_DecodesInitResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/index.php/login/v2" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"login":"https://nc/login/url","poll":{"token":"tok","endpoint":"https://nc/poll"}}`))
	}))
	defer srv.Close()
	info, err := Start(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if info.Login != "https://nc/login/url" {
		t.Errorf("Login: got %q", info.Login)
	}
	if info.Poll.Token != "tok" || info.Poll.Endpoint != "https://nc/poll" {
		t.Errorf("Poll: %+v", info.Poll)
	}
}

func TestStart_NonOKReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()
	if _, err := Start(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error")
	}
}

func TestPoll_RetriesOn404AndReturnsOnSuccess(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n < 3 {
			w.WriteHeader(404)
			return
		}
		_, _ = w.Write([]byte(`{"server":"https://nc","loginName":"alice","appPassword":"secret"}`))
	}))
	defer srv.Close()

	// Three 2s polls dominate the runtime, ~6s total. Acceptable for CI.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := Poll(ctx, PollInfo{Token: "tok", Endpoint: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if res.LoginName != "alice" || res.AppPassword != "secret" {
		t.Errorf("result: %+v", res)
	}
	if got := hits.Load(); got < 3 {
		t.Errorf("expected at least 3 polls, got %d", got)
	}
}

func TestPoll_UnexpectedStatusErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := Poll(ctx, PollInfo{Token: "tok", Endpoint: srv.URL}); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestPoll_HonoursContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := Poll(ctx, PollInfo{Token: "tok", Endpoint: srv.URL})
	if err == nil {
		t.Fatal("expected ctx error")
	}
}
