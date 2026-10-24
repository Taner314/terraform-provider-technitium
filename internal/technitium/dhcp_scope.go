package technitium

import (
	"context"
	"net/url"
)

func IsScopeNotFound(err error) bool {
	return IsAPIError(err, "scope was not found") || IsAPIError(err, "no such dhcp scope")
}

func (c *Client) GetScopes(ctx context.Context) ([]DhcpScopeList, error) {
	var out struct {
		Scopes []DhcpScopeList `json:"scopes"`
	}
	if err := c.Call(ctx, "/api/dhcp/scopes/list", nil, &out); err != nil {
		return nil, err
	}
	return out.Scopes, nil
}

func (c *Client) GetScope(name string, ctx context.Context) (DhcpScope, error) {
	params := url.Values{}
	params.Set("name", name)

	var scope DhcpScope
	err := c.Call(ctx, "/api/dhcp/scopes/get", params, &scope)
	return scope, err
}

// SetScope creates or updates scope name with params (API-named). Params not
// given keep their current value; reserved leases are never touched unless
// "reservedLeases" is passed.
func (c *Client) SetScope(ctx context.Context, name string, params url.Values) error {
	params.Set("name", name)
	return c.Call(ctx, "/api/dhcp/scopes/set", params, nil)
}

func (c *Client) EnableScope(ctx context.Context, name string, enabled bool) error {
	params := url.Values{}
	params.Set("name", name)
	op := "disable"
	if enabled {
		op = "enable"
	}
	return c.Call(ctx, "/api/dhcp/scopes/"+op, params, nil)
}

// ScopeEnabled reports the scope's enabled flag, which only the list call returns.
func (c *Client) ScopeEnabled(ctx context.Context, name string) (bool, bool, error) {
	scopes, err := c.GetScopes(ctx)
	if err != nil {
		return false, false, err
	}
	for _, s := range scopes {
		if s.Name == name {
			return s.Enabled, true, nil
		}
	}
	return false, false, nil
}

func (c *Client) DeleteScope(name string, ctx context.Context) error {
	params := url.Values{}
	params.Set("name", name)
	err := c.Call(ctx, "/api/dhcp/scopes/delete", params, nil)
	if IsScopeNotFound(err) {
		return nil
	}
	return err
}
