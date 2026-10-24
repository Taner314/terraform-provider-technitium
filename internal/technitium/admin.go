package technitium

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

type User struct {
	Username              string   `json:"username"`
	DisplayName           string   `json:"displayName"`
	Type                  string   `json:"type"`
	IsSsoUser             bool     `json:"isSsoUser"`
	TotpEnabled           bool     `json:"totpEnabled"`
	Disabled              bool     `json:"disabled"`
	SessionTimeoutSeconds int64    `json:"sessionTimeoutSeconds"`
	MemberOfGroups        []string `json:"memberOfGroups"`
}

type Group struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

type Session struct {
	Username     string `json:"username"`
	PartialToken string `json:"partialToken"`
	Type         string `json:"type"`
	TokenName    string `json:"tokenName"`
}

type APIToken struct {
	Username  string `json:"username"`
	TokenName string `json:"tokenName"`
	Token     string `json:"token"`
}

type Permission struct {
	Name      string `json:"name"`     // groups
	Username  string `json:"username"` // users
	CanView   bool   `json:"canView"`
	CanModify bool   `json:"canModify"`
	CanDelete bool   `json:"canDelete"`
}

type Permissions struct {
	Section          string       `json:"section"`
	SubItem          string       `json:"subItem"`
	UserPermissions  []Permission `json:"userPermissions"`
	GroupPermissions []Permission `json:"groupPermissions"`
}

func IsUserNotFound(err error) bool  { return IsAPIError(err, "no such user") }
func IsGroupNotFound(err error) bool { return IsAPIError(err, "no such group") }

func (c *Client) CreateUser(ctx context.Context, username, password, displayName string) (User, error) {
	params := url.Values{}
	params.Set("user", username)
	params.Set("pass", password)
	addIfSet(params, "displayName", displayName)
	var u User
	err := c.Call(ctx, "/api/admin/users/create", params, &u)
	return u, err
}

func (c *Client) GetUser(ctx context.Context, username string) (User, error) {
	params := url.Values{}
	params.Set("user", username)
	var u User
	err := c.Call(ctx, "/api/admin/users/get", params, &u)
	return u, err
}

// SetUser sends params (API-named) for user.
func (c *Client) SetUser(ctx context.Context, username string, params url.Values) (User, error) {
	params.Set("user", username)
	var u User
	err := c.Call(ctx, "/api/admin/users/set", params, &u)
	return u, err
}

func (c *Client) DeleteUser(ctx context.Context, username string) error {
	params := url.Values{}
	params.Set("user", username)
	err := c.Call(ctx, "/api/admin/users/delete", params, nil)
	if IsUserNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) CreateGroup(ctx context.Context, name, description string) (Group, error) {
	params := url.Values{}
	params.Set("group", name)
	addIfSet(params, "description", description)
	var g Group
	err := c.Call(ctx, "/api/admin/groups/create", params, &g)
	return g, err
}

func (c *Client) GetGroup(ctx context.Context, name string) (Group, error) {
	params := url.Values{}
	params.Set("group", name)
	params.Set("includeUsers", "true")
	var g Group
	err := c.Call(ctx, "/api/admin/groups/get", params, &g)
	return g, err
}

func (c *Client) SetGroup(ctx context.Context, name string, params url.Values) (Group, error) {
	params.Set("group", name)
	var g Group
	err := c.Call(ctx, "/api/admin/groups/set", params, &g)
	return g, err
}

func (c *Client) DeleteGroup(ctx context.Context, name string) error {
	params := url.Values{}
	params.Set("group", name)
	err := c.Call(ctx, "/api/admin/groups/delete", params, nil)
	if IsGroupNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) CreateAPIToken(ctx context.Context, username, tokenName string) (APIToken, error) {
	params := url.Values{}
	params.Set("user", username)
	params.Set("tokenName", tokenName)
	var t APIToken
	err := c.Call(ctx, "/api/admin/sessions/createToken", params, &t)
	return t, err
}

func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	var out struct {
		Sessions []Session `json:"sessions"`
	}
	err := c.Call(ctx, "/api/admin/sessions/list", nil, &out)
	return out.Sessions, err
}

func (c *Client) DeleteSession(ctx context.Context, partialToken string) error {
	params := url.Values{}
	params.Set("partialToken", partialToken)
	err := c.Call(ctx, "/api/admin/sessions/delete", params, nil)
	if IsAPIError(err, "no such session") || IsAPIError(err, "not found") {
		return nil
	}
	return err
}

// permissionsEndpoint picks the server-wide section endpoint or, with zone
// set, the per-zone one.
func permissionsEndpoint(op, zone string) string {
	if zone != "" {
		return "/api/zones/permissions/" + op
	}
	return "/api/admin/permissions/" + op
}

func (c *Client) GetPermissions(ctx context.Context, section, zone string) (Permissions, error) {
	params := url.Values{}
	if zone != "" {
		params.Set("zone", zone)
	} else {
		params.Set("section", section)
	}
	var p Permissions
	err := c.Call(ctx, permissionsEndpoint("get", zone), params, &p)
	return p, err
}

// SetPermissions replaces the whole permission table of a section/zone.
func (c *Client) SetPermissions(ctx context.Context, p Permissions, zone string) error {
	params := url.Values{}
	if zone != "" {
		params.Set("zone", zone)
	} else {
		params.Set("section", p.Section)
	}
	params.Set("userPermissions", encodePermissions(p.UserPermissions, true))
	params.Set("groupPermissions", encodePermissions(p.GroupPermissions, false))
	return c.Call(ctx, permissionsEndpoint("set", zone), params, nil)
}

func encodePermissions(perms []Permission, users bool) string {
	parts := make([]string, 0, len(perms)*4)
	for _, p := range perms {
		name := p.Name
		if users {
			name = p.Username
		}
		parts = append(parts, name, strconv.FormatBool(p.CanView), strconv.FormatBool(p.CanModify), strconv.FormatBool(p.CanDelete))
	}
	// An empty value clears the table ("false" is rejected here).
	return strings.Join(parts, "|")
}
