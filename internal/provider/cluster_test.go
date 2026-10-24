package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func clusterConfig(heartbeat int) string {
	return testProviderConfig + fmt.Sprintf(`
resource "technitium_cluster" "this" {
  cluster_domain             = "cluster-acc.test"
  ip_addresses               = ["10.255.255.1"]
  heartbeat_refresh_interval = %d
}
`, heartbeat)
}

// Single node: initialize, change options in place, import, delete.
func TestAccCluster(t *testing.T) {
	const addr = "technitium_cluster.this"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: clusterConfig(30),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "nodes.#", "1"),
					resource.TestCheckResourceAttr(addr, "nodes.0.type", "Primary"),
					resource.TestCheckResourceAttr(addr, "config_refresh_interval", "900"),
				),
			},
			{ResourceName: addr, ImportState: true, ImportStateId: "cluster", ImportStateVerify: true, ImportStateVerifyIdentifierAttribute: "cluster_domain"},
			{
				Config: clusterConfig(60),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
				},
				Check: resource.TestCheckResourceAttr(addr, "heartbeat_refresh_interval", "60"),
			},
		},
	})
}

// Two nodes; needs a second server reachable from the first. Set
// TECHNITIUM_SECONDARY_HOST (provider URL of the secondary),
// TECHNITIUM_PRIMARY_IP and TECHNITIUM_SECONDARY_IP (addresses the nodes
// use to reach each other). Provider aliases share one provider process in
// acceptance tests, so the Primary is initialized through the client and
// Terraform only drives the Secondary.
func TestAccClusterJoin(t *testing.T) {
	secondary, primaryIP, secondaryIP := os.Getenv("TECHNITIUM_SECONDARY_HOST"), os.Getenv("TECHNITIUM_PRIMARY_IP"), os.Getenv("TECHNITIUM_SECONDARY_IP")
	if secondary == "" || primaryIP == "" || secondaryIP == "" {
		t.Skip("set TECHNITIUM_SECONDARY_HOST, TECHNITIUM_PRIMARY_IP and TECHNITIUM_SECONDARY_IP for the two-node test")
	}
	ctx := context.Background()
	primary := testClient(t)
	state, err := primary.InitCluster(ctx, "cluster-join.test", []string{primaryIP})
	if err != nil {
		t.Fatal(err)
	}
	self, _ := state.Self()
	t.Cleanup(func() { _ = primary.DeleteCluster(ctx, true) })

	config := fmt.Sprintf(`
provider "technitium" {
  host     = %q
  username = "admin"
  password = "password"
}

resource "technitium_cluster_secondary" "node2" {
  primary_node_url          = %q
  primary_node_ip_address   = %q
  primary_node_username     = "admin"
  primary_node_password     = "password"
  ignore_certificate_errors = true
  ip_addresses              = [%q]
}
`, secondary, self.URL, primaryIP, secondaryIP)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: config,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("technitium_cluster_secondary.node2", "cluster_domain", "cluster-join.test"),
				resource.TestCheckResourceAttr("technitium_cluster_secondary.node2", "nodes.#", "2"),
			),
		}},
	})
}
