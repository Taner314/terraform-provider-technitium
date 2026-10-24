package technitium

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
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
