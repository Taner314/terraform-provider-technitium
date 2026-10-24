package technitium

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

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

func (c *Client) SetScope(s DhcpScope, oldName string, ctx context.Context) (DhcpScope, error) {
	params := url.Values{}
	params.Set("name", s.Name)
	params.Set("startingAddress", s.StartingAddress)
	params.Set("endingAddress", s.EndingAddress)
	params.Set("subnetMask", s.SubnetMask)
	params.Set("routerAddress", s.RouterAddress)
	params.Set("useThisDnsServer", boolString(s.UseThisDnsServer))
	params.Set("dnsServers", strings.Join(s.DnsServers, ","))
	params.Set("domainName", s.DomainName)

	if oldName != "" {
		tflog.Debug(ctx, fmt.Sprintf("Renaming scope from %s to %s", oldName, s.Name))
		params.Set("name", oldName)
		params.Set("newName", s.Name)
	} else {
		tflog.Debug(ctx, fmt.Sprintf("Creating new scope %s", s.Name))
	}

	if s.Exclusions != nil {
		exclusions := make([]string, 0, len(s.Exclusions))
		for _, exclusion := range s.Exclusions {
			exclusions = append(exclusions, exclusion.StartingAddress+"|"+exclusion.EndingAddress)
		}
		params.Set("exclusions", strings.Join(exclusions, "|"))
	}

	if err := c.Call(ctx, "/api/dhcp/scopes/set", params, nil); err != nil {
		return DhcpScope{}, err
	}

	return c.GetScope(s.Name, ctx)
}

func (c *Client) DeleteScope(name string, ctx context.Context) error {
	params := url.Values{}
	params.Set("name", name)
	return c.Call(ctx, "/api/dhcp/scopes/delete", params, nil)
}
