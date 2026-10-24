package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccListedZones(t *testing.T) {
	config := testProviderConfig + `
resource "technitium_blocked_zone" "ads" {
  domain = "Ads.Example.net"
}

resource "technitium_blocked_zone" "tracker" {
  domain = "tracker.example.net"
}

resource "technitium_allowed_zone" "cdn" {
  domain = "cdn.example.org"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_blocked_zone.ads", "id", "ads.example.net"),
					resource.TestCheckResourceAttr("technitium_blocked_zone.ads", "domain", "Ads.Example.net"),
				),
			},
			{ResourceName: "technitium_allowed_zone.cdn", ImportState: true, ImportStateId: "cdn.example.org", ImportStateVerify: true},
			{
				// Removing one entry outside Terraform recreates only that one;
				// a parent domain isn't mistaken for the entry.
				PreConfig: func() {
					c := testClient(t)
					if err := c.DeleteListedZone(context.Background(), "blocked", "tracker.example.net"); err != nil {
						t.Fatal(err)
					}
					if err := c.AddListedZone(context.Background(), "blocked", "example.net"); err != nil {
						t.Fatal(err)
					}
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("technitium_blocked_zone.tracker", plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction("technitium_blocked_zone.ads", plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}
