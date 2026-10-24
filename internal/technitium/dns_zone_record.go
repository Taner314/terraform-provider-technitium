package technitium

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (c *Client) GetDnsZoneRecords(domain string, ctx context.Context) ([]DnsZoneRecord, error) {
	url := fmt.Sprintf("%s/api/zones/records/get?domain=%s", c.HostURL, domain)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	params := req.URL.Query()
	params.Add("listZone", "true")
	req.URL.RawQuery = params.Encode()

	body, err := c.doRequest(req, ctx)
	if err != nil {
		return nil, err
	}

	//tflog.Debug(ctx, string(body))

	response := DnsZoneRecordsResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return response.Response.Records, nil
}

func (c *Client) GetDnsZoneRecord(domain string, recordType string, ctx context.Context) (DnsZoneRecord, error) {
	records, err := c.GetDnsZoneRecords(domain, ctx)

	if err != nil {
		return DnsZoneRecord{}, fmt.Errorf("failed to get DNS zone record: %w", err)
	}

	if len(records) == 0 {
		return DnsZoneRecord{}, fmt.Errorf("no DNS zone records found for domain: %s", domain)
	}

	for _, record := range records {
		if record.Type == recordType && record.Name == domain {
			tflog.Debug(ctx, fmt.Sprintf("Found DNS zone record: %+v", record))
			return record, nil
		}
	}

	return DnsZoneRecord{}, fmt.Errorf("no DNS zone record found for domain: %s with type: %s", domain, recordType)

}

func (c *Client) CreateDnsZoneRecord(r DnsZoneRecordCreate, ctx context.Context) error {

	req, err := c.GetRequest("/api/zones/records/add")
	if err != nil {
		return err
	}

	params := req.URL.Query()
	addIfSet(params, "domain", r.Domain)
	addIfSet(params, "type", r.Type)
	addIfSet(params, "zone", r.Zone)
	params.Add("ttl", fmt.Sprintf("%d", r.TTL))
	addIfSet(params, "comments", r.Comments)
	params.Add("expiryTtl", fmt.Sprintf("%d", r.ExpiryTTL))
	addIfSet(params, "ipAddress", r.IPAddress)
	addIfSet(params, "ptr", r.Ptr)
	params.Add("createPtrZone", fmt.Sprintf("%t", r.CreatePtrZone))
	params.Add("updateSvcbHints", fmt.Sprintf("%t", r.UpdateSvcbHints))
	addIfSet(params, "nameServer", r.NameServer)
	addIfSet(params, "cname", r.Cname)
	addIfSet(params, "ptrName", r.PtrName)
	addIfSet(params, "exchange", r.Exchange)
	params.Add("preference", fmt.Sprintf("%d", r.Preference))
	addIfSet(params, "text", r.Text)
	addIfSet(params, "splitText", r.SplitText)
	addIfSet(params, "protocol", r.Protocol)
	addIfSet(params, "forwarder", r.Forwarder)
	params.Add("forwarderPriority", fmt.Sprintf("%d", r.ForwarderPriority))
	params.Add("dnssecValidation", fmt.Sprintf("%t", r.DnssecValidation))
	addIfSet(params, "proxyType", r.ProxyType)
	addIfSet(params, "proxyAddress", r.ProxyAddress)
	params.Add("proxyPort", fmt.Sprintf("%d", r.ProxyPort))
	addIfSet(params, "proxyUsername", r.ProxyUsername)
	addIfSet(params, "proxyPassword", r.ProxyPassword)
	addIfSet(params, "appName", r.AppName)
	addIfSet(params, "classPath", r.ClassPath)
	addIfSet(params, "recordData", r.RecordData)

	req.URL.RawQuery = params.Encode()

	body, err := c.doRequest(req, ctx)
	if err != nil {
		return err
	}

	response := BaseResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	if response.Status != "ok" {
		return fmt.Errorf("failed to create zone record: %s", response.ErrorMessage)
	}

	return nil

}

func (c *Client) UpdateDnsZoneRecord(r DnsZoneRecordUpdate, ctx context.Context) error {
	req, err := c.GetRequest("/api/zones/records/update")
	if err != nil {
		return err
	}

	params := req.URL.Query()
	addIfSet(params, "domain", r.Domain)
	addIfSet(params, "newDomain", r.NewDomain)
	addIfSet(params, "zone", r.Zone)
	addIfSet(params, "type", r.Type)
	params.Add("disable", fmt.Sprintf("%t", r.Disable))
	params.Add("ttl", fmt.Sprintf("%d", r.TTL))
	params.Add("comments", r.Comments)
	params.Add("expiryTtl", fmt.Sprintf("%d", r.ExpiryTTL))
	addIfSet(params, "ipAddress", r.IPAddress)
	addIfSet(params, "newIpAddress", r.NewIPAddress)
	addIfSet(params, "ptr", r.Ptr)
	params.Add("createPtrZone", fmt.Sprintf("%t", r.CreatePtrZone))
	params.Add("updateSvcbHints", fmt.Sprintf("%t", r.UpdateSvcbHints))
	addIfSet(params, "nameServer", r.NameServer)
	addIfSet(params, "newNameServer", r.NewNameServer)
	addIfSet(params, "cname", r.Cname)
	addIfSet(params, "ptrName", r.PtrName)
	addIfSet(params, "newPtrName", r.NewPtrName)
	addIfSet(params, "exchange", r.Exchange)
	addIfSet(params, "newExchange", r.NewExchange)
	params.Add("preference", fmt.Sprintf("%d", r.Preference))
	params.Add("newPreference", fmt.Sprintf("%d", r.NewPreference))
	addIfSet(params, "text", r.Text)
	addIfSet(params, "newText", r.NewText)
	addIfSet(params, "splitText", r.SplitText)
	addIfSet(params, "newSplitText", r.NewSplitText)
	addIfSet(params, "protocol", r.Protocol)
	addIfSet(params, "forwarder", r.Forwarder)
	addIfSet(params, "newForwarder", r.NewForwarder)
	params.Add("forwarderPriority", fmt.Sprintf("%d", r.ForwarderPriority))
	params.Add("dnssecValidation", fmt.Sprintf("%t", r.DnssecValidation))
	addIfSet(params, "proxyType", r.ProxyType)
	addIfSet(params, "proxyAddress", r.ProxyAddress)
	params.Add("proxyPort", fmt.Sprintf("%d", r.ProxyPort))
	addIfSet(params, "proxyUsername", r.ProxyUsername)
	addIfSet(params, "proxyPassword", r.ProxyPassword)
	addIfSet(params, "appName", r.AppName)
	addIfSet(params, "classPath", r.ClassPath)
	addIfSet(params, "recordData", r.RecordData)

	req.URL.RawQuery = params.Encode()

	body, err := c.doRequest(req, ctx)
	if err != nil {
		return err
	}

	response := BaseResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	if response.Status != "ok" {
		return fmt.Errorf("failed to update zone record: %s", response.ErrorMessage)
	}

	return nil
}

func (c *Client) DeleteDnsZoneRecord(r DnsZoneRecordCreate, ctx context.Context) error {

	req, err := c.GetRequest("/api/zones/records/delete")
	if err != nil {
		return err
	}

	params := req.URL.Query()
	addIfSet(params, "domain", r.Domain)
	addIfSet(params, "type", r.Type)
	addIfSet(params, "zone", r.Zone)
	addIfSet(params, "ipAddress", r.IPAddress)
	addIfSet(params, "ptrName", r.PtrName)
	req.URL.RawQuery = params.Encode()

	body, err := c.doRequest(req, ctx)
	if err != nil {
		return err
	}

	response := BaseResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	if response.Status != "ok" {

		if strings.Contains(response.ErrorMessage, "Cannot delete record: no such record exists") {
			return nil
		}

		return fmt.Errorf("failed to delete zone record: %s", response.ErrorMessage)
	}

	return nil
}

// addIfSet skips empty strings: v15+ treats an empty param (e.g. "zone=")
// as an explicit value rather than absent.
func addIfSet(params url.Values, key, value string) {
	if value != "" {
		params.Add(key, value)
	}
}
