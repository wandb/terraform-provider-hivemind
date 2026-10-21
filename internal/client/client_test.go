package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoSendsBearerAndOrgHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sa-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-Org-Id"); got != "org-1" {
			t.Errorf("X-Org-Id = %q", got)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c, err := New(Config{Endpoint: srv.URL + "/", Token: "sa-token", OrgID: "org-1"})
	if err != nil {
		t.Fatal(err)
	}
	var out struct{ OK bool }
	if err := c.Do(context.Background(), http.MethodGet, "/v1/x", nil, &out); err != nil || !out.OK {
		t.Fatalf("Do = %v, %+v", err, out)
	}
}

func TestDoMapsErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, `{"detail":"insufficient_scope"}`, http.StatusForbidden)
	}))
	defer srv.Close()
	c, _ := New(Config{Endpoint: srv.URL, Token: "t"})
	if err := c.Do(context.Background(), http.MethodGet, "/missing", nil, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	var apiErr *APIError
	if err := c.Do(context.Background(), http.MethodGet, "/forbidden", nil, nil); !errors.As(err, &apiErr) || apiErr.Status != 403 {
		t.Fatalf("want 403 APIError, got %v", err)
	}
}

func TestDoReportsEmptyUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c, _ := New(Config{Endpoint: srv.URL, Token: "t"})
	err := c.Do(context.Background(), http.MethodPut, "/v1/forager/sandbox-providers/sparkbox/secrets/FOO", nil, nil)
	if got, want := err.Error(), "hivemind API returned 502 (Bad Gateway) with an empty response body"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestOIDCExchangeIsCached(t *testing.T) {
	var exchanges atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/actions/exchange":
			exchanges.Add(1)
			var body struct {
				IDToken string `json:"id_token"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.IDToken != "id-token" {
				t.Errorf("id_token = %q", body.IDToken)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": "hm-jwt", "expires_at": time.Now().Add(time.Hour).Unix(),
			})
		default:
			if got := r.Header.Get("Authorization"); got != "Bearer hm-jwt" {
				t.Errorf("Authorization = %q", got)
			}
		}
	}))
	defer srv.Close()
	c, _ := New(Config{Endpoint: srv.URL, OIDCToken: StaticOIDC("id-token")})
	for range 3 {
		if err := c.Do(context.Background(), http.MethodGet, "/v1/x", nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if n := exchanges.Load(); n != 1 {
		t.Fatalf("exchanges = %d, want 1", n)
	}
}

func TestGitHubActionsOIDCRequestsAudience(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("audience"); got != "https://hivemind.example" {
			t.Errorf("audience = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer runner-token" {
			t.Errorf("Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"value":"gh-id-token"}`))
	}))
	defer srv.Close()
	tok, err := GitHubActionsOIDC(srv.URL+"/token?api-version=2.0", "runner-token", "https://hivemind.example", nil)(context.Background())
	if err != nil || tok != "gh-id-token" {
		t.Fatalf("got %q, %v", tok, err)
	}
}

func TestNewRequiresExactlyOneCredential(t *testing.T) {
	if _, err := New(Config{Endpoint: "https://x"}); err == nil {
		t.Fatal("expected error without credentials")
	}
	if _, err := New(Config{Endpoint: "https://x", Token: "t", OIDCToken: StaticOIDC("i")}); err == nil {
		t.Fatal("expected error with both credentials")
	}
}

func TestNewRejectsPlaintextRemoteEndpoint(t *testing.T) {
	for endpoint, ok := range map[string]bool{
		"https://hivemind.wandb.tools": true,
		"http://127.0.0.1:8080":        true,
		"http://localhost:8080":        true,
		"http://hivemind.wandb.tools":  false,
		"ftp://hivemind.wandb.tools":   false,
	} {
		_, err := New(Config{Endpoint: endpoint, Token: "t"})
		if (err == nil) != ok {
			t.Errorf("New(%q) error = %v", endpoint, err)
		}
	}
}
