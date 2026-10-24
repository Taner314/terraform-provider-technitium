package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccDnsZone_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig + `
resource "technitium_dns_zone" "test" {
  name = "zone-basic.test"
  type = "Primary"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_dns_zone.test", "id", "zone-basic.test"),
					resource.TestCheckResourceAttr("technitium_dns_zone.test", "disabled", "false"),
					resource.TestCheckResourceAttr("technitium_dns_zone.test", "catalog", ""),
					resource.TestCheckResourceAttr("technitium_dns_zone.test", "dnssec_status", "Unsigned"),
					// Server defaults are read back.
					resource.TestCheckResourceAttr("technitium_dns_zone.test", "query_access", "Allow"),
				),
			},
			{
				ResourceName:      "technitium_dns_zone.test",
				ImportState:       true,
				ImportStateId:     "zone-basic.test",
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccDnsZone_options(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig + `
resource "technitium_dns_zone" "catalog" {
  name = "catalog-opts.test"
  type = "Catalog"
}

resource "technitium_dns_zone" "test" {
  name                     = "zone-opts.test"
  type                     = "Primary"
  catalog                  = technitium_dns_zone.catalog.name
  query_access             = "UseSpecifiedNetworkACL"
  query_access_network_acl = ["10.0.0.0/8", "!10.1.0.0/16"]
  update                   = "Allow"
  update_security_policies = [
    { tsig_key_name = "k1", domain = "zone-opts.test", allowed_types = ["A", "AAAA"] },
    { tsig_key_name = "k1", domain = "*.zone-opts.test", allowed_types = ["ANY"] },
  ]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_dns_zone.test", "catalog", "catalog-opts.test"),
					resource.TestCheckResourceAttr("technitium_dns_zone.test", "query_access_network_acl.1", "!10.1.0.0/16"),
					resource.TestCheckResourceAttr("technitium_dns_zone.test", "update_security_policies.1.domain", "*.zone-opts.test"),
				),
			},
			{
				// Options change in place; clearing lists and catalog works.
				Config: testProviderConfig + `
resource "technitium_dns_zone" "catalog" {
  name = "catalog-opts.test"
  type = "Catalog"
}

resource "technitium_dns_zone" "test" {
  name                     = "zone-opts.test"
  type                     = "Primary"
  catalog                  = ""
  disabled                 = true
  query_access             = "Allow"
  query_access_network_acl = []
  update                   = "Deny"
  update_security_policies = []
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("technitium_dns_zone.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_dns_zone.test", "catalog", ""),
					resource.TestCheckResourceAttr("technitium_dns_zone.test", "disabled", "true"),
					resource.TestCheckResourceAttr("technitium_dns_zone.test", "query_access_network_acl.#", "0"),
					resource.TestCheckResourceAttr("technitium_dns_zone.test", "update_security_policies.#", "0"),
				),
			},
		},
	})
}

func TestAccDnsZone_forwarder(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig + `
resource "technitium_dns_zone" "test" {
  name = "fwd-zone.test"
  type = "Forwarder"
}

resource "technitium_dns_record_fwd" "test" {
  zone      = technitium_dns_zone.test.name
  name      = "@"
  forwarder = "1.1.1.1"
  priority  = 5
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_dns_zone.test", "type", "Forwarder"),
					resource.TestCheckResourceAttr("technitium_dns_record_fwd.test", "protocol", "Udp"),
					resource.TestCheckResourceAttr("technitium_dns_record_fwd.test", "id", "fwd-zone.test/@/Udp 1.1.1.1"),
				),
			},
			{
				ResourceName:            "technitium_dns_record_fwd.test",
				ImportState:             true,
				ImportStateId:           "fwd-zone.test/@/Udp 1.1.1.1",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"proxy_password"},
			},
		},
	})
}
