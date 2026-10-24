package technitium

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

type ClusterNode struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	IPAddresses []string `json:"ipAddresses"`
	Type        string   `json:"type"`
	State       string   `json:"state"`
	Version     string   `json:"version"`
}

type ClusterState struct {
	ClusterInitialized              bool          `json:"clusterInitialized"`
	DnsServerDomain                 string        `json:"dnsServerDomain"`
	ClusterDomain                   string        `json:"clusterDomain"`
	HeartbeatRefreshIntervalSeconds int64         `json:"heartbeatRefreshIntervalSeconds"`
	HeartbeatRetryIntervalSeconds   int64         `json:"heartbeatRetryIntervalSeconds"`
	ConfigRefreshIntervalSeconds    int64         `json:"configRefreshIntervalSeconds"`
	ConfigRetryIntervalSeconds      int64         `json:"configRetryIntervalSeconds"`
	ClusterNodes                    []ClusterNode `json:"clusterNodes"`
}

// Self returns this server's node entry.
func (s ClusterState) Self() (ClusterNode, bool) {
	for _, n := range s.ClusterNodes {
		if n.State == "Self" {
			return n, true
		}
	}
	return ClusterNode{}, false
}

func (c *Client) GetClusterState(ctx context.Context) (ClusterState, error) {
	var out ClusterState
	err := c.Call(ctx, "/api/admin/cluster/state", nil, &out)
	return out, err
}

func (c *Client) InitCluster(ctx context.Context, domain string, ips []string) (ClusterState, error) {
	p := url.Values{}
	p.Set("clusterDomain", domain)
	p.Set("primaryNodeIpAddresses", strings.Join(ips, ","))
	var out ClusterState
	err := c.Call(ctx, "/api/admin/cluster/init", p, &out)
	return out, err
}

func (c *Client) DeleteCluster(ctx context.Context, force bool) error {
	p := url.Values{}
	p.Set("forceDelete", strconv.FormatBool(force))
	return c.Call(ctx, "/api/admin/cluster/primary/delete", p, nil)
}

func (c *Client) SetClusterOptions(ctx context.Context, p url.Values) (ClusterState, error) {
	var out ClusterState
	err := c.Call(ctx, "/api/admin/cluster/primary/setOptions", p, &out)
	return out, err
}

func (c *Client) UpdateNodeIPAddresses(ctx context.Context, ips []string) error {
	p := url.Values{}
	p.Set("ipAddresses", strings.Join(ips, ","))
	return c.Call(ctx, "/api/admin/cluster/updateIpAddress", p, nil)
}

type JoinCluster struct {
	SecondaryNodeIPAddresses []string
	PrimaryNodeURL           string
	PrimaryNodeIPAddress     string
	IgnoreCertificateErrors  bool
	PrimaryNodeUsername      string
	PrimaryNodePassword      string
	PrimaryNodeTotp          string
}

func (c *Client) JoinCluster(ctx context.Context, j JoinCluster) (ClusterState, error) {
	p := url.Values{}
	p.Set("secondaryNodeIpAddresses", strings.Join(j.SecondaryNodeIPAddresses, ","))
	p.Set("primaryNodeUrl", j.PrimaryNodeURL)
	addIfSet(p, "primaryNodeIpAddress", j.PrimaryNodeIPAddress)
	p.Set("ignoreCertificateErrors", strconv.FormatBool(j.IgnoreCertificateErrors))
	p.Set("primaryNodeUsername", j.PrimaryNodeUsername)
	p.Set("primaryNodePassword", j.PrimaryNodePassword)
	addIfSet(p, "primaryNodeTotp", j.PrimaryNodeTotp)
	var out ClusterState
	err := c.Call(ctx, "/api/admin/cluster/initJoin", p, &out)
	return out, err
}

func (c *Client) LeaveCluster(ctx context.Context, force bool) error {
	p := url.Values{}
	p.Set("forceLeave", strconv.FormatBool(force))
	return c.Call(ctx, "/api/admin/cluster/secondary/leave", p, nil)
}
