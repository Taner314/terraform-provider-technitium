package technitium

import (
	"context"
	"fmt"
	"net/url"
)

type SsoGroupMapping struct {
	RemoteGroup string `json:"remoteGroup"`
	LocalGroup  string `json:"localGroup"`
}

type SsoConfig struct {
	Enabled                       bool              `json:"ssoEnabled"`
	Authority                     *string           `json:"ssoAuthority"`
	ClientID                      *string           `json:"ssoClientId"`
	MetadataAddress               *string           `json:"ssoMetadataAddress"`
	Scopes                        []string          `json:"ssoScopes"`
	AllowSignup                   bool              `json:"ssoAllowSignup"`
	AllowSignupOnlyForMappedUsers bool              `json:"ssoAllowSignupOnlyForMappedUsers"`
	GroupMap                      []SsoGroupMapping `json:"ssoGroupMap"`
}

func (c *Client) GetSsoConfig(ctx context.Context) (SsoConfig, error) {
	var out SsoConfig
	err := c.Call(ctx, "/api/admin/sso/get", nil, &out)
	if err != nil && !IsAPIError(err, "") {
		return out, fmt.Errorf("reading SSO config (SSO requires Technitium 15 or later): %w", err)
	}
	return out, err
}

// SetSsoConfig applies params. The server may restart its web service to
// apply them, so this waits until the API answers again.
func (c *Client) SetSsoConfig(ctx context.Context, params url.Values) (SsoConfig, error) {
	var out SsoConfig
	if err := c.Call(ctx, "/api/admin/sso/set", params, &out); err != nil {
		return out, err
	}
	return out, c.WaitRestart(ctx)
}
