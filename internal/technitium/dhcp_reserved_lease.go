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

func (c *Client) DeleteLease(name string, hardwareAddress string, ctx context.Context) error {
	params := url.Values{}
	params.Set("name", name)
	params.Set("hardwareAddress", hardwareAddress)
	return c.Call(ctx, "/api/dhcp/scopes/removeReservedLease", params, nil)
}
