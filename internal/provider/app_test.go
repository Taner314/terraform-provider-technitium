package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

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

// skipWithoutAppStore skips when the DNS App Store is unreachable. The
// server downloads apps itself and hangs for minutes without egress, which
// would also break the tests that follow.
func skipWithoutAppStore(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, "https://download.technitium.com/dns/apps/SplitHorizonApp-v10.zip", nil)
	client := http.DefaultClient
	// Check through the same proxy the server was given.
	if p := os.Getenv("TECHNITIUM_TEST_PROXY"); p != "" {
		u, err := url.Parse(p)
		if err != nil {
			t.Fatalf("TECHNITIUM_TEST_PROXY: %v", err)
		}
		client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(u)}}
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("DNS App Store unreachable: %v", err)
	}
	_ = resp.Body.Close()
}

func TestAccApp(t *testing.T) {
	skipWithoutAppStore(t)
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
