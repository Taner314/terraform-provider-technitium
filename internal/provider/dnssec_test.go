package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func dnssecConfig(curve, nxProof string, ttl, rollover int) string {
	return testProviderConfig + fmt.Sprintf(`
resource "technitium_dns_zone" "z" {
  name = "dnssec-acc.test"
  type = "Primary"
}

resource "technitium_dns_zone_dnssec" "z" {
  zone              = technitium_dns_zone.z.name
  algorithm         = "ECDSA"
  curve             = %q
  nx_proof          = %q
  dnskey_ttl        = %d
  zsk_rollover_days = %d
}
`, curve, nxProof, ttl, rollover)
}

func TestAccDnssec(t *testing.T) {
	const addr = "technitium_dns_zone_dnssec.z"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: dnssecConfig("P256", "NSEC", 86400, 30),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "status", "SignedWithNSEC"),
					resource.TestCheckResourceAttr(addr, "keys.#", "2"),
					resource.TestCheckResourceAttr(addr, "ds_records.0.algorithm", "ECDSAP256SHA256"),
					resource.TestCheckResourceAttrSet(addr, "ds_records.0.digest"),
				),
			},
			{ResourceName: addr, ImportState: true, ImportStateId: "dnssec-acc.test/ECDSA/P256", ImportStateVerify: true},
			{
				// Proof type and rollover change in place. (DNSKEY TTL can only
				// change hours after signing, once the KSK is Ready.)
				Config: dnssecConfig("P256", "NSEC3", 86400, 60),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "status", "SignedWithNSEC3"),
					resource.TestCheckResourceAttr(addr, "zsk_rollover_days", "60"),
				),
			},
			{
				// A new curve re-signs with new keys.
				Config: dnssecConfig("P384", "NSEC3", 3600, 60),
				// New keys: the TTL applies at signing.
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionReplace)},
				},
				Check: resource.TestCheckResourceAttr(addr, "ds_records.0.algorithm", "ECDSAP384SHA384"),
			},
		},
	})
}
