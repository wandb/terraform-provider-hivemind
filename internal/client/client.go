// Package client is a minimal HiveMind API client for the Forager endpoints the
// provider manages.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ErrNotFound is returned for 404 responses so resources can drop themselves from state.
var ErrNotFound = errors.New("not found")

// APIError carries a non-2xx response.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("hivemind API returned %d (%s) with an empty response body", e.Status, http.StatusText(e.Status))
	}
	return fmt.Sprintf("hivemind API returned %d: %s", e.Status, e.Body)
}

// Config describes how to reach and authenticate to HiveMind.
type Config struct {
	Endpoint string
	OrgID    string
	// Token is a static service-account token. Mutually exclusive with OIDCToken.
	Token string
	// OIDCToken returns an OIDC id_token to exchange for a short-lived HiveMind JWT.
	OIDCToken  func(context.Context) (string, error)
	HTTPClient *http.Client
	UserAgent  string
}

// Client talks to the HiveMind API.
type Client struct {
	cfg Config

	mu        sync.Mutex
	jwt       string
	jwtExpiry time.Time
}

// New validates cfg and returns a client.
func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("endpoint is required")
	}
	if (cfg.Token == "") == (cfg.OIDCToken == nil) {
		return nil, errors.New("exactly one of token or OIDC authentication must be configured")
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}
	if u.Scheme != "https" && (u.Scheme != "http" || !isLoopback(u.Hostname())) {
		return nil, errors.New("endpoint must use https (plain http is allowed only for localhost)")
	}
	cfg.Endpoint = strings.TrimRight(cfg.Endpoint, "/")
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{cfg: cfg}, nil
}

func (c *Client) bearer(ctx context.Context) (string, error) {
	if c.cfg.Token != "" {
		return c.cfg.Token, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.jwt != "" && time.Until(c.jwtExpiry) > time.Minute {
		return c.jwt, nil
	}
	idToken, err := c.cfg.OIDCToken(ctx)
	if err != nil {
		return "", fmt.Errorf("obtaining OIDC token: %w", err)
	}
	var out struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if err := c.send(ctx, http.MethodPost, "/v1/auth/actions/exchange", "", map[string]string{"id_token": idToken}, &out); err != nil {
		return "", fmt.Errorf("exchanging OIDC token: %w", err)
	}
	if out.Token == "" {
		return "", errors.New("exchanging OIDC token: response had no token")
	}
	c.jwt, c.jwtExpiry = out.Token, time.Unix(out.ExpiresAt, 0)
	return c.jwt, nil
}

// Do sends an authenticated JSON request and decodes a JSON response into out (if non-nil).
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	token, err := c.bearer(ctx)
	if err != nil {
		return err
	}
	return c.send(ctx, method, path, token, body, out)
}

func (c *Client) send(ctx context.Context, method, path, token string, body, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.Endpoint+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if c.cfg.OrgID != "" {
		req.Header.Set("X-Org-Id", c.cfg.OrgID)
	}
	if c.cfg.UserAgent != "" {
		req.Header.Set("User-Agent", c.cfg.UserAgent)
	}
	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &APIError{Status: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// GitHubActionsOIDC fetches an id_token from the GitHub Actions runtime.
func GitHubActionsOIDC(requestURL, requestToken, audience string, hc *http.Client) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		u, err := url.Parse(requestURL)
		if err != nil {
			return "", err
		}
		if audience != "" {
			q := u.Query()
			q.Set("audience", audience)
			u.RawQuery = q.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+requestToken)
		if hc == nil {
			hc = http.DefaultClient
		}
		resp, err := hc.Do(req)
		if err != nil {
			return "", err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("GitHub OIDC endpoint returned %d", resp.StatusCode)
		}
		var out struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return "", err
		}
		if out.Value == "" {
			return "", errors.New("GitHub OIDC endpoint returned no token")
		}
		return out.Value, nil
	}
}

// StaticOIDC returns a fixed id_token.
func StaticOIDC(token string) func(context.Context) (string, error) {
	return func(context.Context) (string, error) { return token, nil }
}
