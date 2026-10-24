package provider

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// Every area adopts cleanly with no attributes set: the apply must not change
// anything and the following plan must be empty.
func TestAccSettings_adoptAll(t *testing.T) {
	var cfg strings.Builder
	cfg.WriteString(testProviderConfig)
	for _, s := range settingsSpecs {
		cfg.WriteString(`resource "technitium_settings_` + s.name + `" "this" {}` + "\n")
	}
	steps := []resource.TestStep{{Config: cfg.String()}}
	for _, s := range settingsSpecs {
		addr := "technitium_settings_" + s.name + ".this"
		steps = append(steps, resource.TestStep{
			ResourceName: addr, ImportState: true, ImportStateId: "settings", ImportStateVerify: true,
			ImportStateVerifyIgnore: []string{"tls_certificate_password", "password"},
		})
	}
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: steps})
}

func TestAccSettings_update(t *testing.T) {
	config := func(recursion, forwarders, blockingType, qpm string) string {
		return testProviderConfig + `
resource "technitium_settings_recursion" "this" {
  recursion   = "` + recursion + `"
  network_acl = ["10.0.0.0/8", "!10.66.0.0/16"]
}

resource "technitium_settings_forwarders" "this" {
  forwarders = ` + forwarders + `
}

resource "technitium_settings_blocking" "this" {
  blocking_type             = "` + blockingType + `"
  custom_blocking_addresses = ["0.0.0.0"]
}

resource "technitium_settings_rate_limiting" "this" {
  ipv4_prefix_limits = ` + qpm + `
}

resource "technitium_settings_proxy" "this" {
  type   = "Http"
  address = "10.0.0.9"
  port    = 3128
  bypass  = ["127.0.0.0/8", "localhost"]
}

resource "technitium_settings_general" "this" {
  default_record_ttl         = 1800
  default_responsible_person = "hostmaster@example.net"
  socket_pool_excluded_ports = [53443, 5353]
}
`
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("UseSpecifiedNetworkACL", `["1.1.1.1", "9.9.9.9"]`, "CustomAddress", `[{ prefix = 32, udp_limit = 100, tcp_limit = 200 }]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_settings_recursion.this", "network_acl.1", "!10.66.0.0/16"),
					resource.TestCheckResourceAttr("technitium_settings_forwarders.this", "forwarders.#", "2"),
					resource.TestCheckResourceAttr("technitium_settings_proxy.this", "bypass.1", "localhost"),
					resource.TestCheckResourceAttr("technitium_settings_rate_limiting.this", "ipv4_prefix_limits.0.tcp_limit", "200"),
					resource.TestCheckResourceAttr("technitium_settings_general.this", "socket_pool_excluded_ports.1", "5353"),
				),
			},
			{
				// Clearing lists and switching back.
				Config: config("AllowOnlyForPrivateNetworks", `[]`, "NxDomain", `[]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("technitium_settings_forwarders.this", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("technitium_settings_proxy.this", plancheck.ResourceActionNoop),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_settings_forwarders.this", "forwarders.#", "0"),
					resource.TestCheckResourceAttr("technitium_settings_rate_limiting.this", "ipv4_prefix_limits.#", "0"),
				),
			},
			{
				// Protocol alone is sent together with the existing forwarders.
				Config: testProviderConfig + `
resource "technitium_settings_forwarders" "this" {
  forwarders = ["1.1.1.1"]
}`,
			},
			{
				Config: testProviderConfig + `
resource "technitium_settings_forwarders" "this" {
  protocol = "Tls"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_settings_forwarders.this", "protocol", "Tls"),
					// Unconfigured: follows the server, which added the TLS port.
					resource.TestCheckResourceAttr("technitium_settings_forwarders.this", "forwarders.0", "1.1.1.1:853"),
				),
			},
			{
				// Configured without the default port: no diff.
				Config: testProviderConfig + `
resource "technitium_settings_forwarders" "this" {
  forwarders = ["1.1.1.1", "dns.quad9.net (9.9.9.9)"]
  protocol   = "Tls"
}`,
				Check: resource.TestCheckResourceAttr("technitium_settings_forwarders.this", "forwarders.1", "dns.quad9.net (9.9.9.9)"),
			},
			{
				Config: testProviderConfig + `
resource "technitium_settings_forwarders" "this" {
  forwarders = []
  protocol   = "Udp"
}`,
			},
			{
				// Turn the proxy off so other tests aren't affected.
				Config: testProviderConfig + `
resource "technitium_settings_proxy" "this" {
  type = "None"
}`,
				Check: resource.TestCheckResourceAttr("technitium_settings_proxy.this", "type", "None"),
			},
		},
	})
}
