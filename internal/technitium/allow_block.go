package technitium

import (
	"context"
	"net/url"
	"strings"
)

// List is "allowed" or "blocked".

func (c *Client) AddListedZone(ctx context.Context, list, domain string) error {
	params := url.Values{}
	params.Set("domain", domain)
	return c.Call(ctx, "/api/"+list+"/add", params, nil)
}

func (c *Client) DeleteListedZone(ctx context.Context, list, domain string) error {
	params := url.Values{}
	params.Set("domain", domain)
	return c.Call(ctx, "/api/"+list+"/delete", params, nil)
}

// IsListedZone reports whether domain itself is an entry of the list. The
// list call browses the tree, so membership means it returns the domain with
// its own SOA/NS record.
func (c *Client) IsListedZone(ctx context.Context, list, domain string) (bool, error) {
	params := url.Values{}
	params.Set("domain", domain)
	var out struct {
		Domain  string   `json:"domain"`
		Records []Record `json:"records"`
	}
	if err := c.Call(ctx, "/api/"+list+"/list", params, &out); err != nil {
		return false, err
	}
	want := strings.ToLower(strings.TrimSuffix(domain, "."))
	if !strings.EqualFold(out.Domain, want) {
		return false, nil
	}
	for _, r := range out.Records {
		if strings.EqualFold(r.Name, want) && (r.Type == "SOA" || r.Type == "NS") {
			return true, nil
		}
	}
	return false, nil
}
