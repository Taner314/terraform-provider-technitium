package technitium

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type App struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	DnsApps []struct {
		ClassPath                 string `json:"classPath"`
		IsAppRecordRequestHandler bool   `json:"isAppRecordRequestHandler"`
	} `json:"dnsApps"`
}

type StoreApp struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	URL              string `json:"url"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion"`
	UpdateAvailable  bool   `json:"updateAvailable"`
}

func (c *Client) ListApps(ctx context.Context) ([]App, error) {
	var out struct {
		Apps []App `json:"apps"`
	}
	err := c.Call(ctx, "/api/apps/list", nil, &out)
	return out.Apps, err
}

// GetApp returns the installed app named name.
func (c *Client) GetApp(ctx context.Context, name string) (App, bool, error) {
	apps, err := c.ListApps(ctx)
	if err != nil {
		return App{}, false, err
	}
	for _, a := range apps {
		if strings.EqualFold(a.Name, name) {
			return a, true, nil
		}
	}
	return App{}, false, nil
}

func (c *Client) ListStoreApps(ctx context.Context) ([]StoreApp, error) {
	var out struct {
		StoreApps []StoreApp `json:"storeApps"`
	}
	err := c.Call(ctx, "/api/apps/listStoreApps", nil, &out)
	return out.StoreApps, err
}

// StoreAppURL resolves a store app's download URL.
func (c *Client) StoreAppURL(ctx context.Context, name string) (string, error) {
	apps, err := c.ListStoreApps(ctx)
	if err != nil {
		return "", fmt.Errorf("reading the DNS app store: %w", err)
	}
	for _, a := range apps {
		if strings.EqualFold(a.Name, name) {
			return a.URL, nil
		}
	}
	return "", fmt.Errorf("no app named %q in the DNS app store", name)
}

// DownloadApp installs (or with update, updates) an app from an https URL.
func (c *Client) DownloadApp(ctx context.Context, name, zipURL string, update bool) error {
	params := url.Values{}
	params.Set("name", name)
	params.Set("url", zipURL)
	op := "downloadAndInstall"
	if update {
		op = "downloadAndUpdate"
	}
	return c.CallLong(ctx, "/api/apps/"+op, params, nil)
}

func (c *Client) UninstallApp(ctx context.Context, name string) error {
	params := url.Values{}
	params.Set("name", name)
	err := c.Call(ctx, "/api/apps/uninstall", params, nil)
	if IsAPIError(err, "not found") || IsAPIError(err, "does not exist") {
		return nil
	}
	return err
}

func (c *Client) GetAppConfig(ctx context.Context, name string) (string, error) {
	params := url.Values{}
	params.Set("name", name)
	var out struct {
		Config *string `json:"config"`
	}
	if err := c.Call(ctx, "/api/apps/config/get", params, &out); err != nil {
		return "", err
	}
	if out.Config == nil {
		return "", nil
	}
	return *out.Config, nil
}

func (c *Client) SetAppConfig(ctx context.Context, name, config string) error {
	params := url.Values{}
	params.Set("name", name)
	params.Set("config", config)
	return c.Call(ctx, "/api/apps/config/set", params, nil)
}
