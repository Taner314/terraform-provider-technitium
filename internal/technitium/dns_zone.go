package technitium

import (
	"context"
	"net/url"
	"strings"
)

type DnsZoneList struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Disabled     bool   `json:"disabled"`
	DnssecStatus string `json:"dnssecStatus"`
	SoaSerial    int    `json:"soaSerial"`
	Expiry       string `json:"expiry,omitempty"`
	IsExpired    bool   `json:"isExpired,omitempty"`
	LastModified string `json:"lastModified"`
	Internal     bool   `json:"internal"`
	Catalog      string `json:"catalog"`
}

type UpdateSecurityPolicy struct {
	TsigKeyName  string   `json:"tsigKeyName"`
	Domain       string   `json:"domain"`
	AllowedTypes []string `json:"allowedTypes"`
}

// DnsZone is the response of /api/zones/options/get. Pointer fields are only
// present for some zone types.
type DnsZone struct {
	Name                               string                 `json:"name"`
	Type                               string                 `json:"type"`
	Internal                           bool                   `json:"internal"`
	Disabled                           bool                   `json:"disabled"`
	DnssecStatus                       string                 `json:"dnssecStatus"`
	NotifyFailed                       bool                   `json:"notifyFailed"`
	NotifyFailedFor                    []string               `json:"notifyFailedFor"`
	Catalog                            *string                `json:"catalog"`
	OverrideCatalogQueryAccess         *bool                  `json:"overrideCatalogQueryAccess"`
	OverrideCatalogZoneTransfer        *bool                  `json:"overrideCatalogZoneTransfer"`
	OverrideCatalogNotify              *bool                  `json:"overrideCatalogNotify"`
	PrimaryNameServerAddresses         []string               `json:"primaryNameServerAddresses"`
	PrimaryZoneTransferProtocol        *string                `json:"primaryZoneTransferProtocol"`
	PrimaryZoneTransferTsigKeyName     *string                `json:"primaryZoneTransferTsigKeyName"`
	ValidateZone                       *bool                  `json:"validateZone"`
	QueryAccess                        *string                `json:"queryAccess"`
	QueryAccessNetworkACL              []string               `json:"queryAccessNetworkACL"`
	ZoneTransfer                       *string                `json:"zoneTransfer"`
	ZoneTransferNetworkACL             []string               `json:"zoneTransferNetworkACL"`
	ZoneTransferTsigKeyNames           []string               `json:"zoneTransferTsigKeyNames"`
	Notify                             *string                `json:"notify"`
	NotifyNameServers                  []string               `json:"notifyNameServers"`
	NotifySecondaryCatalogsNameServers []string               `json:"notifySecondaryCatalogsNameServers"`
	Update                             *string                `json:"update"`
	UpdateNetworkACL                   []string               `json:"updateNetworkACL"`
	UpdateSecurityPolicies             []UpdateSecurityPolicy `json:"updateSecurityPolicies"`
}

type DnsZoneCreate struct {
	Name                       string
	Type                       string
	Catalog                    string
	UseSoaSerialDateScheme     bool
	PrimaryNameServerAddresses []string
	ZoneTransferProtocol       string
	TsigKeyName                string
	ValidateZone               bool
	Protocol                   string
	Forwarder                  string
	InitializeForwarder        bool
	DnssecValidation           bool
}

func (c *Client) GetDnsZones(ctx context.Context) ([]DnsZoneList, error) {
	var out struct {
		Zones []DnsZoneList `json:"zones"`
	}
	if err := c.Call(ctx, "/api/zones/list", nil, &out); err != nil {
		return nil, err
	}
	return out.Zones, nil
}

func (c *Client) GetDnsZone(name string, ctx context.Context) (DnsZone, error) {
	params := url.Values{}
	params.Set("zone", name)

	var zone DnsZone
	err := c.Call(ctx, "/api/zones/options/get", params, &zone)
	return zone, err
}

// IsZoneNotFound reports whether err means the zone does not exist.
func IsZoneNotFound(err error) bool {
	return IsAPIError(err, "no such zone") || IsAPIError(err, "zone was not found")
}

func (c *Client) CreateDnsZone(z DnsZoneCreate, ctx context.Context) (string, error) {
	params := url.Values{}
	params.Set("zone", z.Name)
	params.Set("type", z.Type)
	addIfSet(params, "catalog", z.Catalog)
	if z.UseSoaSerialDateScheme {
		params.Set("useSoaSerialDateScheme", "true")
	}
	addIfSet(params, "primaryNameServerAddresses", strings.Join(z.PrimaryNameServerAddresses, ","))
	addIfSet(params, "zoneTransferProtocol", z.ZoneTransferProtocol)
	addIfSet(params, "tsigKeyName", z.TsigKeyName)
	if z.ValidateZone {
		params.Set("validateZone", "true")
	}
	if z.Type == "Forwarder" {
		params.Set("initializeForwarder", boolString(z.InitializeForwarder))
		addIfSet(params, "protocol", z.Protocol)
		addIfSet(params, "forwarder", z.Forwarder)
		if z.DnssecValidation {
			params.Set("dnssecValidation", "true")
		}
	}

	var out struct {
		Domain string `json:"domain"`
	}
	if err := c.Call(ctx, "/api/zones/create", params, &out); err != nil {
		return "", err
	}
	// For reverse zones created from an IP/CIDR the server picks the name.
	if out.Domain == "" {
		out.Domain = z.Name
	}
	return out.Domain, nil
}

// SetDnsZoneOptions sends params (already API-named) to /api/zones/options/set.
func (c *Client) SetDnsZoneOptions(ctx context.Context, zone string, params url.Values) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("zone", zone)
	return c.Call(ctx, "/api/zones/options/set", params, nil)
}

func (c *Client) DeleteDnsZone(name string, ctx context.Context) error {
	params := url.Values{}
	params.Set("zone", name)
	err := c.Call(ctx, "/api/zones/delete", params, nil)
	if IsZoneNotFound(err) {
		return nil
	}
	return err
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
