package technitium

import (
	"context"
	"net/url"
)

// Record is a resource record as returned by /api/zones/records/*. RData is
// kept untyped because its shape depends on Type.
type Record struct {
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	TTL          int64          `json:"ttl"`
	Disabled     bool           `json:"disabled"`
	Comments     string         `json:"comments"`
	ExpiryTTL    int64          `json:"expiryTtl"`
	DnssecStatus string         `json:"dnssecStatus"`
	LastModified string         `json:"lastModified"`
	RData        map[string]any `json:"rData"`
}

// GetRecords returns the records owned by domain in zone. With listZone it
// returns every record in the zone instead.
func (c *Client) GetRecords(ctx context.Context, zone, domain string, listZone bool) ([]Record, error) {
	params := url.Values{}
	params.Set("domain", domain)
	addIfSet(params, "zone", zone)
	if listZone {
		params.Set("listZone", "true")
	}

	var out struct {
		Records []Record `json:"records"`
	}
	if err := c.Call(ctx, "/api/zones/records/get", params, &out); err != nil {
		return nil, err
	}
	return out.Records, nil
}

// AddRecord adds a record. params must carry domain, zone, type and the
// type-specific values.
func (c *Client) AddRecord(ctx context.Context, params url.Values) (Record, error) {
	var out struct {
		AddedRecord Record `json:"addedRecord"`
	}
	err := c.Call(ctx, "/api/zones/records/add", params, &out)
	return out.AddedRecord, err
}

// UpdateRecord updates a record identified by its current values in params,
// with new values in the new* params.
func (c *Client) UpdateRecord(ctx context.Context, params url.Values) (Record, error) {
	var out struct {
		UpdatedRecord Record `json:"updatedRecord"`
	}
	err := c.Call(ctx, "/api/zones/records/update", params, &out)
	return out.UpdatedRecord, err
}

// DeleteRecord deletes a record identified by params. A record that is
// already gone is not an error.
func (c *Client) DeleteRecord(ctx context.Context, params url.Values) error {
	err := c.Call(ctx, "/api/zones/records/delete", params, nil)
	if IsAPIError(err, "no such record") || IsAPIError(err, "no such zone") {
		return nil
	}
	return err
}
