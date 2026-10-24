package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func dhcpConfig(scopeName, scopeBody, leaseIP, leaseHost string) string {
	return testProviderConfig + fmt.Sprintf(`
resource "technitium_dhcp_scope" "lan" {
  name             = %q
  starting_address = "10.30.0.10"
  ending_address   = "10.30.0.200"
  subnet_mask      = "255.255.255.0"
  %s
}

resource "technitium_dhcp_reserved_lease" "printer" {
  scope            = technitium_dhcp_scope.lan.name
  hardware_address = "00:11:22:aa:bb:cc"
  ip_address       = %q
  host_name        = %q
  comments         = "office printer"
}

resource "technitium_dhcp_reserved_lease" "nas" {
  scope            = technitium_dhcp_scope.lan.name
  hardware_address = "00-11-22-33-44-55"
  ip_address       = "10.30.0.6"
}

data "technitium_dhcp_scope" "lan" {
  name       = technitium_dhcp_scope.lan.name
  depends_on = [technitium_dhcp_scope.lan]
}

data "technitium_dhcp_scopes" "all" {
  depends_on = [technitium_dhcp_scope.lan]
}
`, scopeName, scopeBody, leaseIP, leaseHost)
}

const fullScope = `
  enabled            = true
  lease_time_days    = 0
  lease_time_hours   = 12
  router_address     = "10.30.0.1"
  domain_name        = "lan.test"
  domain_search_list = ["lan.test", "home.arpa"]
  dns_servers        = ["10.30.0.1", "1.1.1.1"]
  ntp_servers        = ["10.30.0.1"]
  ping_check_enabled = true
  exclusions = [
    { starting_address = "10.30.0.100", ending_address = "10.30.0.110" },
  ]
  static_routes = [
    { destination = "172.16.0.0", subnet_mask = "255.255.0.0", router = "10.30.0.2" },
  ]
  generic_options = [
    { code = 150, value = "0A:1E:00:01" },
  ]
`

func TestAccDhcp(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: dhcpConfig("lan-acc", fullScope, "10.30.0.5", "printer"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "enabled", "true"),
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "lease_time_hours", "12"),
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "dns_servers.#", "2"),
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "dns_servers.0", "10.30.0.1"),
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "exclusions.0.ending_address", "10.30.0.110"),
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "static_routes.0.router", "10.30.0.2"),
					// Defaults from the server are read back.
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "dns_ttl", "900"),
					// MAC kept in configured spelling.
					resource.TestCheckResourceAttr("technitium_dhcp_reserved_lease.printer", "hardware_address", "00:11:22:aa:bb:cc"),
					resource.TestCheckResourceAttr("technitium_dhcp_reserved_lease.printer", "id", "lan-acc/00:11:22:aa:bb:cc"),
					resource.TestCheckResourceAttr("data.technitium_dhcp_scope.lan", "router_address", "10.30.0.1"),
					resource.TestCheckResourceAttr("data.technitium_dhcp_scope.lan", "dns_servers.#", "2"),
					resource.TestCheckResourceAttrSet("data.technitium_dhcp_scopes.all", "scopes.#"),
				),
			},
			{ResourceName: "technitium_dhcp_scope.lan", ImportState: true, ImportStateId: "lan-acc", ImportStateVerify: true},
			{
				ResourceName: "technitium_dhcp_reserved_lease.printer", ImportState: true,
				ImportStateId: "lan-acc/00:11:22:aa:bb:cc", ImportStateVerify: true,
			},
			{
				// Lease changes re-create the reservation; the scope is untouched.
				Config: dhcpConfig("lan-acc", fullScope, "10.30.0.7", "printer2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("technitium_dhcp_reserved_lease.printer", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("technitium_dhcp_scope.lan", plancheck.ResourceActionNoop),
					},
				},
				Check: resource.TestCheckResourceAttr("technitium_dhcp_reserved_lease.printer", "ip_address", "10.30.0.7"),
			},
			{
				// Scope rename and option removal in place; reserved leases survive.
				Config: dhcpConfig("lan-renamed", `enabled = false`, "10.30.0.7", "printer2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("technitium_dhcp_scope.lan", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "id", "lan-renamed"),
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "enabled", "false"),
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "dns_servers.#", "0"),
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "router_address", ""),
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "exclusions.#", "0"),
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "static_routes.#", "0"),
					// Unset tuned values keep their last value.
					resource.TestCheckResourceAttr("technitium_dhcp_scope.lan", "lease_time_hours", "12"),
					resource.TestCheckResourceAttr("technitium_dhcp_reserved_lease.printer", "scope", "lan-renamed"),
					checkLeaseCount("lan-renamed", 2),
				),
			},
			{
				// A reservation deleted outside Terraform is recreated.
				PreConfig: func() {
					if err := testClient(t).DeleteLease("lan-renamed", "00-11-22-33-44-55", context.Background()); err != nil {
						t.Fatal(err)
					}
				},
				Config: dhcpConfig("lan-renamed", `enabled = false`, "10.30.0.7", "printer2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("technitium_dhcp_reserved_lease.nas", plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction("technitium_dhcp_reserved_lease.printer", plancheck.ResourceActionNoop),
					},
				},
			},
			{
				// A scope deleted outside Terraform is recreated with its leases.
				PreConfig: func() {
					if err := testClient(t).DeleteScope("lan-renamed", context.Background()); err != nil {
						t.Fatal(err)
					}
				},
				Config: dhcpConfig("lan-renamed", `enabled = false`, "10.30.0.7", "printer2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("technitium_dhcp_scope.lan", plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction("technitium_dhcp_reserved_lease.printer", plancheck.ResourceActionCreate),
					},
				},
				Check: checkLeaseCount("lan-renamed", 2),
			},
		},
	})
}

// checkLeaseCount asserts the server-side reservation count, proving scope
// updates don't drop leases owned by other resources.
func checkLeaseCount(scope string, want int) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		host := os.Getenv("TECHNITIUM_HOST")
		if host == "" {
			host = "http://localhost:5380"
		}
		c, err := technitium.New(context.Background(), technitium.Config{Host: host, Username: "admin", Password: "password"})
		if err != nil {
			return err
		}
		s, err := c.GetScope(scope, context.Background())
		if err != nil {
			return err
		}
		if len(s.ReservedLeases) != want {
			return fmt.Errorf("scope %s has %d reserved leases, want %d", scope, len(s.ReservedLeases), want)
		}
		return nil
	}
}
