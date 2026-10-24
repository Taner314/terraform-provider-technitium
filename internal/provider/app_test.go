package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// Needs the server to reach the DNS App Store (download.technitium.com).
func appConfig(preference int) string {
	return testProviderConfig + fmt.Sprintf(`
resource "technitium_app" "split" {
  name = "Split Horizon"
  config = jsonencode({
    appPreference            = %d
    enableAddressTranslation = false
    networks                 = { lab = ["10.40.0.0/16"] }
    domainGroupMap           = {}
    networkGroupMap          = {}
    groups                   = []
  })
}

resource "technitium_dns_zone" "z" {
  name = "app-acc.test"
  type = "Primary"
}

resource "technitium_dns_record_app" "www" {
  zone        = technitium_dns_zone.z.name
  name        = "www"
  app_name    = technitium_app.split.name
  class_path  = "SplitHorizon.SimpleAddress"
  record_data = jsonencode({ public = ["203.0.113.10"], private = ["10.40.0.10"] })
}
`, preference)
}

func TestAccApp(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: appConfig(40),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("technitium_app.split", "version"),
					resource.TestCheckTypeSetElemAttr("technitium_app.split", "class_paths.*", "SplitHorizon.SimpleAddress"),
				),
			},
			{ResourceName: "technitium_app.split", ImportState: true, ImportStateId: "Split Horizon", ImportStateVerify: true,
				// Imported config is the server's formatting; equal as JSON.
				ImportStateVerifyIgnore: []string{"config"}},
			{
				Config: appConfig(50),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("technitium_app.split", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("technitium_dns_record_app.www", plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}
