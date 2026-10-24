package technitium

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	HostURL    string
	HTTPClient HttpClient
	Token      string
	// longClient serves calls where the server does slow work (downloads,
	// signing, cluster sync); nil falls back to HTTPClient.
	longClient HttpClient
}

// LongTimeout bounds calls made with CallLong.
const LongTimeout = 10 * time.Minute

type Config struct {
	Host               string
	Token              string
	Username           string
	Password           string
	Timeout            time.Duration
	InsecureSkipVerify bool
}

// APIError is a non-"ok" status returned by the Technitium API.
type APIError struct {
	Endpoint string
	Status   string
	Message  string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("%s: status %q", e.Endpoint, e.Status)
	}
	return fmt.Sprintf("%s: %s", e.Endpoint, e.Message)
}

// IsAPIError reports whether err is an APIError whose message contains substr
// (case-insensitive).
func IsAPIError(err error, substr string) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return strings.Contains(strings.ToLower(apiErr.Message), strings.ToLower(substr))
}

type envelope struct {
	Status       string          `json:"status"`
	ErrorMessage string          `json:"errorMessage"`
	Response     json.RawMessage `json:"response"`
}

// New builds a client, logging in with username/password when no token is
// given, and verifies the session.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if def, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = def.Clone()
	}
	if cfg.InsecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in for self-signed homelab certs
	}

	c := &Client{
		HostURL:    strings.TrimRight(cfg.Host, "/"),
		HTTPClient: &http.Client{Timeout: cfg.Timeout, Transport: transport},
		Token:      cfg.Token,
		longClient: &http.Client{Timeout: LongTimeout, Transport: transport},
	}

	if c.Token == "" {
		if cfg.Username == "" || cfg.Password == "" {
			return nil, fmt.Errorf("either a token or username and password must be provided")
		}
		token, err := c.Login(ctx, cfg.Username, cfg.Password)
		if err != nil {
			return nil, err
		}
		c.Token = token
	}

	if err := c.GetSessionInfo(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

// NewClient is kept for callers that already hold a token.
func NewClient(host, token string, ctx context.Context) (*Client, error) {
	return New(ctx, Config{Host: host, Token: token})
}

// GetToken logs in and returns a session token.
func GetToken(host string, username string, password string) (string, error) {
	if username == "" || password == "" {
		return "", fmt.Errorf("username and password must be provided")
	}
	c := &Client{HostURL: strings.TrimRight(host, "/"), HTTPClient: &http.Client{Timeout: 30 * time.Second}}
	return c.Login(context.Background(), username, password)
}

func (c *Client) Login(ctx context.Context, username, password string) (string, error) {
	params := url.Values{}
	params.Set("user", username)
	params.Set("pass", password)

	var result struct {
		Token string `json:"token"`
	}
	// The login call returns the token at the top level, not under "response".
	raw, err := c.post(ctx, c.HTTPClient, "/api/user/login", params)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("decoding login response: %w", err)
	}
	if result.Token == "" {
		return "", fmt.Errorf("token not found in login response")
	}
	return result.Token, nil
}

func (c *Client) GetSessionInfo(ctx context.Context) error {
	if c.Token == "" {
		return fmt.Errorf("missing API token")
	}
	return c.Call(ctx, "/api/user/session/get", nil, nil)
}

// Call POSTs params as a form to endpoint and decodes the "response" object
// into out (which may be nil).
func (c *Client) Call(ctx context.Context, endpoint string, params url.Values, out any) error {
	return c.call(ctx, c.HTTPClient, endpoint, params, out)
}

// CallLong is Call for slow server-side operations.
func (c *Client) CallLong(ctx context.Context, endpoint string, params url.Values, out any) error {
	hc := c.longClient
	if hc == nil {
		hc = c.HTTPClient
	}
	return c.call(ctx, hc, endpoint, params, out)
}

// WaitReady polls until the API answers, e.g. after a web service restart.
func (c *Client) WaitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := c.GetSessionInfo(ctx)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("web service did not come back: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (c *Client) call(ctx context.Context, hc HttpClient, endpoint string, params url.Values, out any) error {
	raw, err := c.post(ctx, hc, endpoint, params)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("%s: decoding response: %w", endpoint, err)
	}
	if len(env.Response) == 0 || string(env.Response) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Response, out); err != nil {
		return fmt.Errorf("%s: decoding response: %w", endpoint, err)
	}
	return nil
}

// post performs the request, checks the status envelope and returns the raw
// body.
func (c *Client) post(ctx context.Context, hc HttpClient, endpoint string, params url.Values) ([]byte, error) {
	form := url.Values{}
	for k, v := range params {
		form[k] = v
	}
	// v15+ reads the Bearer header; v14 only reads the token param. Both go
	// in the body/headers, never the URL.
	if c.Token != "" {
		form.Set("token", c.Token)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.HostURL+endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	tflog.Debug(ctx, "technitium API call", map[string]any{"endpoint": endpoint, "params": paramNames(params)})

	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d: %s", endpoint, res.StatusCode, truncate(string(body), 512))
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("%s: decoding response: %w", endpoint, err)
	}
	if env.Status != "ok" {
		return nil, &APIError{Endpoint: endpoint, Status: env.Status, Message: env.ErrorMessage}
	}

	return body, nil
}

// paramNames lists param keys only, so values such as passwords never reach
// the logs.
func paramNames(params url.Values) []string {
	names := make([]string, 0, len(params))
	for k := range params {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// addIfSet skips empty strings: v15+ treats an empty param (e.g. "zone=")
// as an explicit value rather than absent.
func addIfSet(params url.Values, key, value string) {
	if value != "" {
		params.Add(key, value)
	}
}
