package technitium

import (
	"context"
	"net/url"
	"strings"
	"sync"
)

// Settings is the raw /api/settings/get response; its shape varies by
// server version, so it's kept untyped.
type Settings map[string]any

type TsigKey struct {
	KeyName       string `json:"keyName"`
	SharedSecret  string `json:"sharedSecret"`
	AlgorithmName string `json:"algorithmName"`
}

// settingsMu serialises read-modify-write of server-wide lists (TSIG keys)
// within one provider process; Terraform applies resources in parallel.
var settingsMu sync.Mutex

func (c *Client) GetSettings(ctx context.Context) (Settings, error) {
	var s Settings
	err := c.Call(ctx, "/api/settings/get", nil, &s)
	return s, err
}

// SetSettings applies params and returns the resulting settings.
func (c *Client) SetSettings(ctx context.Context, params url.Values) (Settings, error) {
	var s Settings
	err := c.Call(ctx, "/api/settings/set", params, &s)
	return s, err
}

func (c *Client) GetTsigKeys(ctx context.Context) ([]TsigKey, error) {
	var out struct {
		TsigKeys []TsigKey `json:"tsigKeys"`
	}
	err := c.Call(ctx, "/api/settings/get", nil, &out)
	return out.TsigKeys, err
}

// ModifyTsigKeys applies fn to the current key list and writes it back.
func (c *Client) ModifyTsigKeys(ctx context.Context, fn func([]TsigKey) []TsigKey) ([]TsigKey, error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	keys, err := c.GetTsigKeys(ctx)
	if err != nil {
		return nil, err
	}
	keys = fn(keys)

	params := url.Values{}
	if len(keys) == 0 {
		params.Set("tsigKeys", "false")
	} else {
		parts := make([]string, 0, len(keys)*3)
		for _, k := range keys {
			parts = append(parts, k.KeyName, k.SharedSecret, k.AlgorithmName)
		}
		params.Set("tsigKeys", strings.Join(parts, "|"))
	}

	var out struct {
		TsigKeys []TsigKey `json:"tsigKeys"`
	}
	err = c.Call(ctx, "/api/settings/set", params, &out)
	return out.TsigKeys, err
}

// LockSettings serialises read-modify-write of other shared settings tables.
func LockSettings() func() {
	settingsMu.Lock()
	return settingsMu.Unlock
}
