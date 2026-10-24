package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func adminConfig(displayName, groups, groupName, password string, canModify bool) string {
	return testProviderConfig + fmt.Sprintf(`
resource "technitium_group" "ops" {
  name        = %q
  description = "Operations"
}

resource "technitium_user" "bob" {
  username     = "bob"
  password     = %q
  display_name = %q
  groups       = %s
}

resource "technitium_permission" "ops_zones" {
  section    = "Zones"
  group      = technitium_group.ops.name
  can_view   = true
  can_modify = %t
}

resource "technitium_dns_zone" "z" {
  name = "perm-acc.test"
  type = "Primary"
}

resource "technitium_zone_permission" "bob_zone" {
  zone       = technitium_dns_zone.z.name
  user       = technitium_user.bob.username
  can_modify = true
}

resource "technitium_api_token" "ci" {
  user = technitium_user.bob.username
  name = "ci"
}

resource "technitium_tsig_key" "xfr" {
  name = "xfr.acc.test"
}

resource "technitium_tsig_key" "fixed" {
  name      = "fixed.acc.test"
  algorithm = "hmac-sha512"
  secret    = "c2VjcmV0c2VjcmV0c2VjcmV0c2VjcmV0"
}
`, groupName, password, displayName, groups, canModify)
}

func TestAccAdmin(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: adminConfig("Bob", `[technitium_group.ops.name, "DNS Administrators"]`, "ops", "secret-1", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_user.bob", "groups.#", "2"),
					resource.TestCheckResourceAttr("technitium_user.bob", "session_timeout_seconds", "1800"),
					resource.TestMatchResourceAttr("technitium_api_token.ci", "token", regexp.MustCompile(`^[0-9a-f]{64}$`)),
					resource.TestCheckResourceAttr("technitium_permission.ops_zones", "id", "Zones/group/ops"),
					resource.TestCheckResourceAttr("technitium_zone_permission.bob_zone", "id", "perm-acc.test/user/bob"),
					resource.TestCheckResourceAttrSet("technitium_tsig_key.xfr", "secret"),
					resource.TestCheckResourceAttr("technitium_tsig_key.xfr", "algorithm", "hmac-sha256"),
					resource.TestCheckResourceAttr("technitium_tsig_key.fixed", "secret", "c2VjcmV0c2VjcmV0c2VjcmV0c2VjcmV0"),
				),
			},
			{ResourceName: "technitium_group.ops", ImportState: true, ImportStateId: "ops", ImportStateVerify: true,
				// members is refreshed after the user joins.
				ImportStateVerifyIgnore: []string{"members"}},
			{ResourceName: "technitium_user.bob", ImportState: true, ImportStateId: "bob", ImportStateVerify: true, ImportStateVerifyIgnore: []string{"password"}},
			{ResourceName: "technitium_permission.ops_zones", ImportState: true, ImportStateId: "Zones/group/ops", ImportStateVerify: true},
			{ResourceName: "technitium_zone_permission.bob_zone", ImportState: true, ImportStateId: "perm-acc.test/user/bob", ImportStateVerify: true},
			{ResourceName: "technitium_tsig_key.xfr", ImportState: true, ImportStateId: "xfr.acc.test", ImportStateVerify: true},
			{
				// In place: group rename, user display name, password and
				// membership, permission flags.
				Config: adminConfig("Robert", `["DNS Administrators"]`, "operations", "secret-2", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("technitium_group.ops", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("technitium_user.bob", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("technitium_api_token.ci", plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction("technitium_tsig_key.xfr", plancheck.ResourceActionNoop),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("technitium_user.bob", "display_name", "Robert"),
					resource.TestCheckResourceAttr("technitium_user.bob", "groups.#", "1"),
					resource.TestCheckResourceAttr("technitium_group.ops", "id", "operations"),
					resource.TestCheckResourceAttr("technitium_permission.ops_zones", "can_modify", "true"),
				),
			},
		},
	})
}

func TestAccUser_everyoneRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfig + `
resource "technitium_user" "x" {
  username = "x"
  password = "x"
  groups   = ["Everyone"]
}`,
				ExpectError: regexp.MustCompile(`Everyone`),
			},
		},
	})
}
