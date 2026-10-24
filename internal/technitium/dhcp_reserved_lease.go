package technitium

import (
	"context"
	"net/url"
)

func (c *Client) CreateLease(s DhcpReservedLease, ctx context.Context) error {
	params := url.Values{}
	params.Set("name", s.Name)
	params.Set("hardwareAddress", s.HardwareAddress)
	params.Set("ipAddress", s.IpAddress)
	addIfSet(params, "hostName", s.HostName)
	addIfSet(params, "comments", s.Comments)
	return c.Call(ctx, "/api/dhcp/scopes/addReservedLease", params, nil)
}

// DeleteLease removes a reserved lease; a missing lease or scope is not an error.
func (c *Client) DeleteLease(name string, hardwareAddress string, ctx context.Context) error {
	params := url.Values{}
	params.Set("name", name)
	params.Set("hardwareAddress", hardwareAddress)
	err := c.Call(ctx, "/api/dhcp/scopes/removeReservedLease", params, nil)
	if IsAPIError(err, "no such reserved lease") || IsScopeNotFound(err) {
		return nil
	}
	return err
}

type DhcpLease struct {
	Scope            string `json:"scope"`
	Type             string `json:"type"`
	HardwareAddress  string `json:"hardwareAddress"`
	ClientIdentifier string `json:"clientIdentifier"`
	Address          string `json:"address"`
	HostName         string `json:"hostName"`
	LeaseObtained    string `json:"leaseObtained"`
	LeaseExpires     string `json:"leaseExpires"`
}

func (c *Client) ListLeases(ctx context.Context) ([]DhcpLease, error) {
	var out struct {
		Leases []DhcpLease `json:"leases"`
	}
	err := c.Call(ctx, "/api/dhcp/leases/list", nil, &out)
	return out.Leases, err
}
