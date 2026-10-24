package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

var (
	// testAccProtoV6ProviderFactories are used to instantiate a provider during
	// acceptance testing. The factory function will be invoked for every Terraform
	// CLI command executed to create a provider server to which the CLI can
	// reattach.
	testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		// A fresh provider per call: aliases must not share one instance.
		"technitium": func() (tfprotov6.ProviderServer, error) {
			return providerserver.NewProtocol6WithError(New("test")())()
		},
	}
)

func TestProviderAuthWithUsernameAndPassword(t *testing.T) {

	providerConfig := `
provider "technitium" {
  username = "admin"
  password = "password"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "technitium_dns_zone" "auth" {
  name = "auth-test.example"
  type = "Primary"
}

data "technitium_dns_zones" "test" {
  depends_on = [technitium_dns_zone.auth]
}`,
				// v15+ hides internal zones from the list, so seed one
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.technitium_dns_zones.test", "zones.#"),
				),
			},
		},
	})

}
