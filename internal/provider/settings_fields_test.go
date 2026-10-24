package provider

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Fields that would cut off the test server or need host resources.
var settingsWriteSkip = map[string]bool{
	"general.local_end_points": true, "general.ipv4_source_addresses": true, "general.ipv6_source_addresses": true,
	"web_service.local_addresses": true, "web_service.http_port": true, "web_service.enable_tls": true,
	"web_service.enable_http3": true, "web_service.http_to_tls_redirect": true, "web_service.tls_port": true,
	"web_service.use_self_signed_tls_certificate": true, "web_service.tls_certificate_path": true,
	"web_service.enable_http_unix_socket": true, "web_service.http_unix_socket": true,
	"web_service.enable_tls_unix_socket": true, "web_service.tls_unix_socket": true,
	"protocols.tls_certificate_path": true, "protocols.dns_over_http_unix_socket": true,
	"protocols.dns_over_https_unix_socket": true, "protocols.enable_dns_over_http_unix_socket": true,
	"protocols.enable_dns_over_https_unix_socket": true,
	// TLS listeners need a certificate.
	"protocols.enable_dns_over_tls": true, "protocols.enable_dns_over_https": true,
	"protocols.enable_dns_over_http3": true, "protocols.enable_dns_over_quic": true,
	"logging.log_folder": true,
	// Covered by TestAccSettings_update (needs related fields set together).
	"proxy.type": true, "proxy.address": true, "proxy.port": true, "proxy.username": true, "proxy.bypass": true,
	"forwarders.protocol":              true,
	"rate_limiting.ipv4_prefix_limits": true, "rate_limiting.ipv6_prefix_limits": true,
}

// Valid alternative values for enum-like and formatted string fields.
var settingsWriteStrings = map[string]string{
	"general.dns_server_domain":                "acc-test-server",
	"general.default_responsible_person":       "hostmaster@example.net",
	"general.ipv6_mode":                        "Enabled",
	"general.edns_client_subnet_ipv4_override": "10.1.0.0/24",
	"general.edns_client_subnet_ipv6_override": "2001:db8::/56",
	"web_service.real_ip_header":               "X-Forwarded-For",
	"web_service.csp_frame_ancestors":          "'self'",
	"protocols.real_ip_header":                 "X-Forwarded-For",
	"recursion.recursion":                      "Deny",
	"blocking.blocking_type":                   "AnyAddress",
	"forwarders.protocol":                      "Tcp",
	"logging.logging_type":                     "Console",
}

// Fields whose default is their maximum.
var atMax = map[string]bool{"cache.serve_stale_max_wait_time": true}

var settingsWriteLists = map[string]string{
	"blocking.block_list_urls":           "https://example.invalid/hosts.txt",
	"blocking.custom_blocking_addresses": "0.0.0.0",
	"forwarders.forwarders":              "9.9.9.9",
}

// TestAccSettings_everyFieldWrites proves each field's API param name and
// encoding: set a different value, read it back, restore.
func TestAccSettings_everyFieldWrites(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	ctx := context.Background()
	c := testClient(t)

	for _, spec := range settingsSpecs {
		for _, f := range spec.fields {
			name := spec.name + "." + f.attr
			if f.writeOnly || settingsWriteSkip[name] {
				continue
			}
			t.Run(name, func(t *testing.T) {
				before, err := c.GetSettings(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if _, _, ok := f.resolve(before); !ok {
					t.Skipf("%s not available on Technitium %s", f.key, serverVersion(before))
				}
				cur, _ := f.fromSettings(before)
				alt, ok := alternative(f, name, cur)
				if !ok {
					t.Fatalf("no test value for %s; add one to settingsWriteStrings", name)
				}

				set := func(v attr.Value) attr.Value {
					enc, _ := f.encode(ctx, v)
					after, err := c.SetSettings(ctx, url.Values{f.key: {enc}})
					if err != nil {
						t.Fatalf("set %s=%q: %v", f.key, enc, err)
					}
					got, _ := f.fromSettings(after)
					return got
				}

				key, _, _ := f.resolve(before)
				f.key = key
				if got := set(alt); !got.Equal(alt) {
					t.Errorf("after setting %s: got %s, want %s", f.key, got, alt)
				}
				if got := set(cur); !got.Equal(cur) {
					t.Errorf("restoring %s: got %s, want %s", f.key, got, cur)
				}
			})
		}
	}
}

func alternative(f settingField, name string, cur attr.Value) (attr.Value, bool) {
	switch f.kind {
	case sBool:
		return types.BoolValue(!cur.(types.Bool).ValueBool()), true
	case sInt:
		n := cur.(types.Int64).ValueInt64()
		if atMax[name] {
			return types.Int64Value(n - 1), true
		}
		return types.Int64Value(n + 1), true
	case sIntList:
		return types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(5353)}), true
	case sStrList:
		v := "10.9.0.0/16"
		if s, ok := settingsWriteLists[name]; ok {
			v = s
		}
		return types.ListValueMust(types.StringType, []attr.Value{types.StringValue(v)}), true
	case sStr:
		v, ok := settingsWriteStrings[name]
		return types.StringValue(v), ok
	}
	return nil, false
}

func TestNormForwarder(t *testing.T) {
	for in, want := range map[string]string{
		"1.1.1.1:853":                          "1.1.1.1",
		"1.1.1.1":                              "1.1.1.1",
		"dns.quad9.net:853 (9.9.9.9)":          "dns.quad9.net (9.9.9.9)",
		"dns.quad9.net (9.9.9.9:853)":          "dns.quad9.net (9.9.9.9)",
		"[2606:4700::1111]:53":                 "[2606:4700::1111]",
		"10.0.0.1:5353":                        "10.0.0.1:5353",
		"https://cloudflare-dns.com/dns-query": "https://cloudflare-dns.com/dns-query",
	} {
		if got := normForwarder(in); got != want {
			t.Errorf("normForwarder(%q) = %q, want %q", in, got, want)
		}
	}
}
