package technitium

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"syscall"
	"testing"
	"time"
)

type captured struct {
	method, path, auth, contentType string
	form                            url.Values
}

// fakeServer answers every request with body and records the last request.
func fakeServer(t *testing.T, status int, body string) (*Client, *captured) {
	t.Helper()
	got := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		got.method, got.path = r.Method, r.URL.Path
		got.auth, got.contentType = r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		got.form = r.PostForm
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "tok"}, got
}

func TestCallSendsBearerAndForm(t *testing.T) {
	c, got := fakeServer(t, http.StatusOK, `{"status":"ok","response":{"domain":"example.com"}}`)

	params := url.Values{}
	params.Set("zone", "example.com")
	var out struct {
		Domain string `json:"domain"`
	}
	if err := c.Call(context.Background(), "/api/zones/create", params, &out); err != nil {
		t.Fatal(err)
	}

	if got.method != http.MethodPost || got.path != "/api/zones/create" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.auth != "Bearer tok" {
		t.Errorf("Authorization = %q", got.auth)
	}
	if got.contentType != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", got.contentType)
	}
	if got.form.Get("zone") != "example.com" || got.form.Get("token") != "tok" {
		t.Errorf("form = %v", got.form)
	}
	if out.Domain != "example.com" {
		t.Errorf("decoded domain = %q", out.Domain)
	}
}

func TestCallErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
		apiErr  bool
	}{
		{"api error", 200, `{"status":"error","errorMessage":"No such zone was found: x"}`, "No such zone", true},
		{"invalid token", 200, `{"status":"invalid-token","errorMessage":"Invalid token or session expired."}`, "Invalid token", true},
		{"http error", 502, `bad gateway`, "HTTP 502", false},
		{"bad json", 200, `<html>`, "decoding response", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := fakeServer(t, tt.status, tt.body)
			err := c.Call(context.Background(), "/api/x", nil, nil)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
			if got := IsAPIError(err, ""); got != tt.apiErr {
				t.Errorf("IsAPIError = %v, want %v", got, tt.apiErr)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	c, got := fakeServer(t, http.StatusOK, `{"status":"ok","token":"abc","isSsoUser":false,"info":{"permissions":{}}}`)
	c.Token = ""

	token, err := c.Login(context.Background(), "admin", "p@ss&word")
	if err != nil {
		t.Fatal(err)
	}
	if token != "abc" {
		t.Errorf("token = %q", token)
	}
	if got.form.Get("user") != "admin" || got.form.Get("pass") != "p@ss&word" {
		t.Errorf("form = %v", got.form)
	}
	if got.auth != "" {
		t.Errorf("login sent Authorization %q", got.auth)
	}
}

func TestDeleteRecordIgnoresMissing(t *testing.T) {
	c, _ := fakeServer(t, http.StatusOK, `{"status":"error","errorMessage":"Cannot delete record: no such record exists."}`)
	if err := c.DeleteRecord(context.Background(), url.Values{}); err != nil {
		t.Errorf("DeleteRecord = %v, want nil", err)
	}
}

func TestParamNamesHidesValues(t *testing.T) {
	params := url.Values{"pass": {"secret"}, "user": {"admin"}}
	names := paramNames(params)
	if strings.Join(names, ",") != "pass,user" {
		t.Errorf("paramNames = %v", names)
	}
}

func TestCallRetriesWhileServerRestarts(t *testing.T) {
	restartPoll = 50 * time.Millisecond
	defer func() { restartPoll = 2 * time.Second }()

	// Reserve a port, close it so connections are refused, then start the
	// server on it shortly after.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	go func() {
		time.Sleep(300 * time.Millisecond)
		l2, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"status":"ok","response":{}}`))
		})}
		t.Cleanup(func() { _ = srv.Close() })
		_ = srv.Serve(l2)
	}()

	c := &Client{HostURL: "http://" + addr, HTTPClient: &http.Client{Timeout: 5 * time.Second}, Token: "tok"}
	if err := c.Call(context.Background(), "/api/x", nil, nil); err != nil {
		t.Fatalf("Call after restart = %v", err)
	}
}

func TestRetryable(t *testing.T) {
	reset := &net.OpError{Op: "read", Err: syscall.ECONNRESET}
	refused := &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
	for _, tt := range []struct {
		err      error
		endpoint string
		want     bool
	}{
		{refused, "/api/zones/records/add", true},
		{reset, "/api/user/login", true},
		{reset, "/api/zones/records/get", true},
		{reset, "/api/zones/records/add", false},
		{errors.New("boom"), "/api/zones/records/get", false},
	} {
		if got := retryable(tt.err, tt.endpoint); got != tt.want {
			t.Errorf("retryable(%v, %s) = %v, want %v", tt.err, tt.endpoint, got, tt.want)
		}
	}
}
