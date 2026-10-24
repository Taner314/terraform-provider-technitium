package technitium

import (
	"context"
	"net/url"
	"strconv"
)

type DnssecKey struct {
	KeyTag       int64  `json:"keyTag"`
	KeyType      string `json:"keyType"`
	Algorithm    string `json:"algorithm"`
	State        string `json:"state"`
	IsRetiring   bool   `json:"isRetiring"`
	RolloverDays int64  `json:"rolloverDays"`
}

type DnssecProperties struct {
	DnssecStatus    string      `json:"dnssecStatus"`
	Nsec3Iterations *int64      `json:"nsec3Iterations"`
	Nsec3SaltLength *int64      `json:"nsec3SaltLength"`
	DnsKeyTtl       int64       `json:"dnsKeyTtl"`
	PrivateKeys     []DnssecKey `json:"dnssecPrivateKeys"`
}

type DsRecord struct {
	KeyTag    int64  `json:"keyTag"`
	State     string `json:"dnsKeyState"`
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"publicKey"`
	Digests   []struct {
		DigestType string `json:"digestType"`
		Digest     string `json:"digest"`
	} `json:"digests"`
}

func zoneParam(zone string) url.Values {
	p := url.Values{}
	p.Set("zone", zone)
	return p
}

func (c *Client) SignZone(ctx context.Context, zone string, params url.Values) error {
	params.Set("zone", zone)
	return c.Call(ctx, "/api/zones/dnssec/sign", params, nil)
}

func (c *Client) UnsignZone(ctx context.Context, zone string) error {
	err := c.Call(ctx, "/api/zones/dnssec/unsign", zoneParam(zone), nil)
	if IsZoneNotFound(err) || IsAPIError(err, "not signed") {
		return nil
	}
	return err
}

func (c *Client) GetDnssecProperties(ctx context.Context, zone string) (DnssecProperties, error) {
	var out DnssecProperties
	err := c.Call(ctx, "/api/zones/dnssec/properties/get", zoneParam(zone), &out)
	return out, err
}

func (c *Client) GetDsRecords(ctx context.Context, zone string) ([]DsRecord, error) {
	var out struct {
		DsRecords []DsRecord `json:"dsRecords"`
	}
	err := c.Call(ctx, "/api/zones/dnssec/viewDS", zoneParam(zone), &out)
	return out.DsRecords, err
}

// SetNxProof converts between NSEC and NSEC3.
func (c *Client) SetNxProof(ctx context.Context, zone, proof string) error {
	op := "convertToNSEC"
	if proof == "NSEC3" {
		op = "convertToNSEC3"
	}
	return c.Call(ctx, "/api/zones/dnssec/properties/"+op, zoneParam(zone), nil)
}

func (c *Client) UpdateNsec3Params(ctx context.Context, zone string, iterations, saltLength int64) error {
	p := zoneParam(zone)
	p.Set("iterations", strconv.FormatInt(iterations, 10))
	p.Set("saltLength", strconv.FormatInt(saltLength, 10))
	return c.Call(ctx, "/api/zones/dnssec/properties/updateNSEC3Params", p, nil)
}

func (c *Client) UpdateDnsKeyTtl(ctx context.Context, zone string, ttl int64) error {
	p := zoneParam(zone)
	p.Set("ttl", strconv.FormatInt(ttl, 10))
	return c.Call(ctx, "/api/zones/dnssec/properties/updateDnsKeyTtl", p, nil)
}

func (c *Client) UpdatePrivateKeyRollover(ctx context.Context, zone string, keyTag, days int64) error {
	p := zoneParam(zone)
	p.Set("keyTag", strconv.FormatInt(keyTag, 10))
	p.Set("rolloverDays", strconv.FormatInt(days, 10))
	return c.Call(ctx, "/api/zones/dnssec/properties/updatePrivateKey", p, nil)
}
