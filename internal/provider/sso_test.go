package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func skipBelow15(t *testing.T) {
	t.Helper()
	s, err := testClient(t).GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := s["version"].(string); strings.HasPrefix(v, "14.") {
		t.Skipf("SSO needs Technitium 15+, server is %s", v)
	}
}

func ssoConfig(groupMap string) string {
	return testProviderConfig + `
resource "technitium_sso" "this" {
  enabled           = false
  authority         = "https://auth.example.invalid/application/o/technitium/"
  client_id         = "technitium"
  client_secret     = "s3cret"
  additional_scopes = ["email", "groups"]
  group_map         = ` + groupMap + `
}
`
}

func TestAccSso(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { skipBelow15(t) },
		Steps: []resource.TestStep{
			{
				Config: ssoConfig(`[{ remote_group = "dns-admins", local_group = "DNS Administrators" }]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_sso.this", "additional_scopes.1", "groups"),
					resource.TestCheckResourceAttr("technitium_sso.this", "group_map.0.local_group", "DNS Administrators"),
				),
			},
			{ResourceName: "technitium_sso.this", ImportState: true, ImportStateId: "sso", ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{"client_secret"}},
			{
				Config: ssoConfig(`[]`),
				Check:  resource.TestCheckResourceAttr("technitium_sso.this", "group_map.#", "0"),
			},
		},
	})
}
