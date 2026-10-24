package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const testProviderConfig = `
provider "technitium" {
  username = "admin"
  password = "password"
}
`

type recordCase struct {
	typ      string
	zoneType string // default Primary
	name     string
	body     string // type-specific attributes
	updated  string // body after an in-place change; "" skips the step
	replace  bool   // the update forces replacement instead
}

// Values are deliberately spelled differently from what the server stores
// (case, trailing dots, uncompressed IPv6, lowercase hex) to prove plans stay
// empty after normalisation.
var recordCases = []recordCase{
	{typ: "A", name: "www", body: `ip_address = "10.0.0.1"`, updated: `ip_address = "10.0.0.2"`},
	{typ: "AAAA", name: "v6", body: `ip_address = "2001:DB8:0:0::1"`, updated: `ip_address = "2001:db8::2"`},
	{typ: "NS", name: "sub", body: `name_server = "NS1.Example.net."`, updated: `name_server = "ns2.example.net"`},
	{typ: "CNAME", name: "alias", body: `cname = "Target.Example.net."`, updated: `cname = "other.example.net"`},
	{typ: "PTR", name: "ptr", body: `ptr_name = "host.example.net."`, updated: `ptr_name = "host2.example.net"`},
	{typ: "MX", name: "@", body: "preference = 10\n exchange = \"mail.example.net.\"", updated: "preference = 20\n exchange = \"mail.example.net\""},
	{typ: "TXT", name: "txt", body: `text = "v=spf1 include:example.net -all"`, updated: `text = "hello world"`},
	{typ: "RP", name: "rp", body: "mailbox = \"admin@example.net\"\n txt_domain = \"info.example.net\"", updated: "mailbox = \"ops@example.net\"\n txt_domain = \"info.example.net\""},
	{typ: "SRV", name: "_sip._tcp", body: "priority = 1\n weight = 2\n port = 5060\n target = \"sip.example.net\"", updated: "priority = 1\n weight = 2\n port = 5061\n target = \"sip.example.net\""},
	{typ: "NAPTR", name: "naptr", body: "order = 100\n preference = 10\n flags = \"U\"\n services = \"E2U+sip\"\n regexp = \"!^.*$!sip:info@example.net!\"\n replacement = \".\"",
		updated: "order = 100\n preference = 20\n flags = \"U\"\n services = \"E2U+sip\"\n regexp = \"!^.*$!sip:info@example.net!\"\n replacement = \".\""},
	{typ: "DNAME", name: "dname", body: `dname = "example.net"`, updated: `dname = "example.org"`},
	{typ: "DS", name: "sub", body: "key_tag = 12345\n algorithm = \"ECDSAP256SHA256\"\n digest_type = \"SHA256\"\n digest = \"bbe017b17e5cb5ffff1ec2c7815367df80d8e7eaee4832d3ed192159d79b1eeb\"",
		updated: "key_tag = 12346\n algorithm = \"ECDSAP256SHA256\"\n digest_type = \"SHA256\"\n digest = \"bbe017b17e5cb5ffff1ec2c7815367df80d8e7eaee4832d3ed192159d79b1eeb\""},
	{typ: "SSHFP", name: "host", body: "algorithm = \"Ed25519\"\n fingerprint_type = \"SHA256\"\n fingerprint = \"aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899\"",
		updated: "algorithm = \"Ed25519\"\n fingerprint_type = \"SHA256\"\n fingerprint = \"00bbccddeeff00112233445566778899aabbccddeeff00112233445566778899\""},
	{typ: "TLSA", name: "_443._tcp", body: "certificate_usage = \"DANE-EE\"\n selector = \"SPKI\"\n matching_type = \"SHA2-256\"\n certificate_association_data = \"aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899\"",
		updated: "certificate_usage = \"DANE-TA\"\n selector = \"SPKI\"\n matching_type = \"SHA2-256\"\n certificate_association_data = \"aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899\""},
	{typ: "SVCB", name: "_dns", body: "priority = 1\n target_name = \"svc.example.net.\"\n params = { alpn = \"h2,h3\", port = \"8443\" }",
		updated: "priority = 1\n target_name = \"svc.example.net\"\n params = { alpn = \"h2\" }"},
	{typ: "HTTPS", name: "@", body: "priority = 1\n target_name = \".\"\n params = { alpn = \"h2\" }\n auto_ipv4_hint = true",
		updated: "priority = 2\n target_name = \".\"\n params = { alpn = \"h2\" }\n auto_ipv4_hint = true"},
	{typ: "URI", name: "_ftp._tcp", body: "priority = 10\n weight = 1\n uri = \"ftp://ftp.example.net/public\"", updated: "priority = 10\n weight = 5\n uri = \"ftp://ftp.example.net/public\""},
	{typ: "CAA", name: "@", body: "flags = 0\n tag = \"issue\"\n value = \"letsencrypt.org\"", updated: "flags = 0\n tag = \"issuewild\"\n value = \"letsencrypt.org\""},
	{typ: "ANAME", name: "@", body: `aname = "lb.example.net"`, updated: `aname = "lb2.example.net"`},
	{typ: "FWD", zoneType: "Forwarder", name: "@", body: "forwarder = \"9.9.9.9\"\n protocol = \"Udp\"", updated: "forwarder = \"dns.quad9.net\"\n protocol = \"Tls\"\n priority = 1\n dnssec_validation = true"},
	{typ: "APP", name: "app", body: "app_name = \"x\"\n class_path = \"y\"\n record_data = \"z\"", updated: "app_name = \"x\"\n class_path = \"y\"\n record_data = \"zz\""},
}

func recordConfig(zone string, c recordCase, body, extra string) string {
	zoneType := c.zoneType
	if zoneType == "" {
		zoneType = "Primary"
	}
	return fmt.Sprintf(`%s
resource "technitium_dns_zone" "test" {
  name = %q
  type = %q
}

resource "technitium_dns_record_%s" "test" {
  zone = technitium_dns_zone.test.name
  name = %q
  %s
  %s
}
`, testProviderConfig, zone, zoneType, strings.ToLower(c.typ), c.name, body, extra)
}

func TestAccDnsRecords(t *testing.T) {
	for _, c := range recordCases {
		t.Run(c.typ, func(t *testing.T) {
			zone := "rec-" + strings.ToLower(c.typ) + ".test"
			addr := "technitium_dns_record_" + strings.ToLower(c.typ) + ".test"

			steps := []resource.TestStep{
				{
					Config: recordConfig(zone, c, c.body, `ttl = 600
  comments = "managed by terraform"`),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr(addr, "ttl", "600"),
						resource.TestCheckResourceAttr(addr, "fqdn", fqdn(c.name, zone)),
						resource.TestCheckResourceAttr(addr, "comments", "managed by terraform"),
					),
				},
				{
					ResourceName:            addr,
					ImportState:             true,
					ImportStateIdFunc:       stateID(addr),
					ImportStateVerify:       true,
					ImportStateVerifyIgnore: importIgnore(c),
				},
			}
			if c.updated != "" {
				action := plancheck.ResourceActionUpdate
				if c.replace {
					action = plancheck.ResourceActionReplace
				}
				steps = append(steps, resource.TestStep{
					// Identity change in place; comments removed, disabled.
					Config: recordConfig(zone, c, c.updated, `ttl = 300
  disabled = true`),
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, action)},
					},
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr(addr, "ttl", "300"),
						resource.TestCheckResourceAttr(addr, "comments", ""),
						resource.TestCheckResourceAttr(addr, "disabled", "true"),
					),
				})
			}
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps:                    steps,
			})
		})
	}
}

// Normalised import differs from the configured spelling for these, and
// write-only flags can't be read back.
func importIgnore(c recordCase) []string {
	ignore := []string{"ptr", "create_ptr_zone", "update_svcb_hints", "proxy_password"}
	switch c.typ {
	case "AAAA":
		ignore = append(ignore, "ip_address")
	case "NS":
		ignore = append(ignore, "name_server")
	case "CNAME":
		ignore = append(ignore, "cname")
	case "PTR":
		ignore = append(ignore, "ptr_name")
	case "MX":
		ignore = append(ignore, "exchange")
	case "DS":
		ignore = append(ignore, "digest")
	case "SSHFP":
		ignore = append(ignore, "fingerprint")
	case "TLSA":
		ignore = append(ignore, "certificate_association_data")
	case "SVCB", "HTTPS":
		ignore = append(ignore, "target_name")
	case "NAPTR":
		ignore = append(ignore, "replacement")
	case "FWD":
		ignore = append(ignore, "ttl")
	}
	return ignore
}

func stateID(addr string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return "", fmt.Errorf("%s not in state", addr)
		}
		return rs.Primary.ID, nil
	}
}

// Multiple records of a type at one name are tracked independently.
func TestAccDnsRecord_multipleSameName(t *testing.T) {
	config := testProviderConfig + `
resource "technitium_dns_zone" "test" {
  name = "rec-multi.test"
  type = "Primary"
}

resource "technitium_dns_record_a" "one" {
  zone       = technitium_dns_zone.test.name
  name       = "rr"
  ip_address = "10.0.0.1"
  ttl        = 120
}

resource "technitium_dns_record_a" "two" {
  zone       = technitium_dns_zone.test.name
  name       = "rr"
  ip_address = "10.0.0.2"
  ttl        = 120
}

resource "technitium_dns_record_txt" "spf" {
  zone = technitium_dns_zone.test.name
  name = "@"
  text = "v=spf1 -all"
}

resource "technitium_dns_record_txt" "verify" {
  zone = technitium_dns_zone.test.name
  name = "@"
  text = "google-site-verification=abc"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_dns_record_a.one", "ttl", "120"),
					resource.TestCheckResourceAttr("technitium_dns_record_a.two", "ttl", "120"),
					resource.TestCheckResourceAttr("technitium_dns_record_txt.spf", "ttl", "3600"),
					resource.TestCheckResourceAttr("technitium_dns_record_a.one", "id", "rec-multi.test/rr/10.0.0.1"),
				),
			},
			{
				// Deleting one record outside Terraform only recreates that one.
				PreConfig: func() {
					deleteRecordOutOfBand(t, "rec-multi.test", "rr.rec-multi.test", "A", "ipAddress", "10.0.0.2")
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("technitium_dns_record_a.two", plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction("technitium_dns_record_a.one", plancheck.ResourceActionNoop),
					},
				},
			},
			{
				// An out-of-band TTL change (shared by the RRset) is reverted.
				PreConfig: func() {
					updateRecordOutOfBand(t, "rec-multi.test", "rr.rec-multi.test", "A", "ipAddress", "10.0.0.1", "ttl", "999")
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("technitium_dns_record_a.one", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("technitium_dns_record_txt.spf", plancheck.ResourceActionNoop),
					},
				},
				Check: resource.TestCheckResourceAttr("technitium_dns_record_a.one", "ttl", "120"),
			},
		},
	})
}

func TestAccDnsRecord_rename(t *testing.T) {
	config := func(name string) string {
		return testProviderConfig + fmt.Sprintf(`
resource "technitium_dns_zone" "test" {
  name = "rec-rename.test"
  type = "Primary"
}

resource "technitium_dns_record_cname" "test" {
  zone  = technitium_dns_zone.test.name
  name  = %q
  cname = "target.example.net"
}
`, name)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config("old")},
			{
				Config: config("new"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("technitium_dns_record_cname.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("technitium_dns_record_cname.test", "fqdn", "new.rec-rename.test"),
			},
		},
	})
}

func testClient(t *testing.T) *technitium.Client {
	t.Helper()
	host := os.Getenv("TECHNITIUM_HOST")
	if host == "" {
		host = "http://localhost:5380"
	}
	c, err := technitium.New(context.Background(), technitium.Config{Host: host, Username: "admin", Password: "password"})
	if err != nil {
		t.Fatalf("test client: %v", err)
	}
	return c
}

func deleteRecordOutOfBand(t *testing.T, zone, domain, typ string, kv ...string) {
	t.Helper()
	params := recordParams(zone, domain, typ, kv...)
	if err := testClient(t).Call(context.Background(), "/api/zones/records/delete", params, nil); err != nil {
		t.Fatalf("out-of-band delete: %v", err)
	}
}

func updateRecordOutOfBand(t *testing.T, zone, domain, typ string, kv ...string) {
	t.Helper()
	params := recordParams(zone, domain, typ, kv...)
	if err := testClient(t).Call(context.Background(), "/api/zones/records/update", params, nil); err != nil {
		t.Fatalf("out-of-band update: %v", err)
	}
}

func recordParams(zone, domain, typ string, kv ...string) map[string][]string {
	params := map[string][]string{"zone": {zone}, "domain": {domain}, "type": {typ}}
	for i := 0; i+1 < len(kv); i += 2 {
		params[kv[i]] = []string{kv[i+1]}
	}
	return params
}

func TestAccDnsRecordsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: testProviderConfig + `
resource "technitium_dns_zone" "z" {
  name = "ds-records.test"
  type = "Primary"
}

resource "technitium_dns_record_a" "a" {
  zone       = technitium_dns_zone.z.name
  name       = "www"
  ip_address = "10.0.0.1"
}

resource "technitium_dns_record_mx" "mx" {
  zone       = technitium_dns_zone.z.name
  name       = "@"
  preference = 10
  exchange   = "mail.ds-records.test"
}

data "technitium_dns_records" "www" {
  zone       = technitium_dns_zone.z.name
  name       = "www"
  depends_on = [technitium_dns_record_a.a]
}

data "technitium_dns_records" "mx" {
  zone       = technitium_dns_zone.z.name
  type       = "MX"
  depends_on = [technitium_dns_record_mx.mx]
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.technitium_dns_records.www", "records.#", "1"),
				resource.TestCheckResourceAttr("data.technitium_dns_records.www", "records.0.data.ipAddress", "10.0.0.1"),
				resource.TestCheckResourceAttr("data.technitium_dns_records.www", "records.0.name", "www"),
				resource.TestCheckResourceAttr("data.technitium_dns_records.mx", "records.#", "1"),
				resource.TestCheckResourceAttr("data.technitium_dns_records.mx", "records.0.data.preference", "10"),
				resource.TestCheckResourceAttr("data.technitium_dns_records.mx", "records.0.name", "@"),
			),
		}},
	})
}
