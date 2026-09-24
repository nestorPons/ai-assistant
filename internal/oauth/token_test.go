package oauth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenRefresh(t *testing.T) {
	var calls int32
	var gotForm map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		gotForm = map[string]string{
			"grant_type":    r.PostForm.Get("grant_type"),
			"client_id":     r.PostForm.Get("client_id"),
			"client_secret": r.PostForm.Get("client_secret"),
			"refresh_token": r.PostForm.Get("refresh_token"),
		}
		fmt.Fprintf(w, `{"access_token":"tok-%d","expires_in":3600,"token_type":"Bearer"}`, atomic.LoadInt32(&calls))
	}))
	defer srv.Close()

	ts := New(Config{
		ClientID:     "cid",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		TokenURL:     srv.URL,
	})

	tok1, err := ts.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tok1 != "tok-1" {
		t.Errorf("token = %q, want tok-1", tok1)
	}
	if gotForm["grant_type"] != "refresh_token" || gotForm["client_id"] != "cid" ||
		gotForm["client_secret"] != "secret" || gotForm["refresh_token"] != "refresh" {
		t.Errorf("formulario de refresh inesperado: %v", gotForm)
	}

	tok2, err := ts.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tok2 != tok1 {
		t.Errorf("token cacheado = %q, want %q", tok2, tok1)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("llamadas al endpoint de token = %d, want 1 (cacheado)", got)
	}

	ts.mu.Lock()
	ts.tokenExpiry = time.Now().Add(-time.Minute)
	ts.mu.Unlock()

	tok3, err := ts.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tok3 != "tok-2" {
		t.Errorf("token renovado = %q, want tok-2", tok3)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("llamadas al endpoint de token = %d, want 2", got)
	}
}

func TestTokenStatic(t *testing.T) {
	ts := New(Config{AccessToken: "estatico"})
	tok, err := ts.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tok != "estatico" {
		t.Errorf("token = %q, want estatico", tok)
	}
}

func TestTokenRefreshError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)
	}))
	defer srv.Close()

	ts := New(Config{ClientID: "cid", ClientSecret: "secret", RefreshToken: "malo", TokenURL: srv.URL})

	if _, err := ts.Token(context.Background()); err == nil {
		t.Fatal("esperaba error de refresh")
	} else if !strings.Contains(err.Error(), "expired or revoked") {
		t.Errorf("error inesperado: %v", err)
	}
}
