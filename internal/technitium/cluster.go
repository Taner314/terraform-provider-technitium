package technitium

import (
	"context"
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
