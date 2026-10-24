package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const zonesListResponse = `{"status":"ok","response":{"zones":[
{"name":"0.in-addr.arpa","type":"Primary","lastModified":"2026-10-24T19:58:47.7116736Z","disabled":false,"soaSerial":1,"internal":true,"catalog":null,"dnssecStatus":"Unsigned"},
{"name":"example.com","type":"Primary","lastModified":"2026-10-24T19:58:47.7116736Z","disabled":false,"soaSerial":3,"internal":false,"catalog":null,"dnssecStatus":"Unsigned"}]}}`

func TestDnsZonesDataSource(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/zones/list" {
			_, _ = w.Write([]byte(zonesListResponse))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok","response":{}}`))
	}))
	defer server.Close()

	config := fmt.Sprintf(`
provider "technitium" {
  host = "%s"
  token = "test"
}
`, server.URL)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config + `data "technitium_dns_zones" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.technitium_dns_zones.test", "zones.0.name", "0.in-addr.arpa"),
					resource.TestCheckResourceAttr("data.technitium_dns_zones.test", "zones.0.type", "Primary"),
					resource.TestCheckResourceAttr("data.technitium_dns_zones.test", "zones.0.catalog", ""),
					resource.TestCheckResourceAttr("data.technitium_dns_zones.test", "zones.0.disabled", "false"),
					resource.TestCheckResourceAttr("data.technitium_dns_zones.test", "zones.0.dnssec_status", "Unsigned"),
					resource.TestCheckResourceAttr("data.technitium_dns_zones.test", "zones.0.soa_serial", "1"),
					resource.TestCheckResourceAttr("data.technitium_dns_zones.test", "zones.0.expiry", ""),
					resource.TestCheckResourceAttr("data.technitium_dns_zones.test", "zones.0.is_expired", "false"),
					resource.TestCheckResourceAttr("data.technitium_dns_zones.test", "zones.0.internal", "true"),
					resource.TestCheckResourceAttr("data.technitium_dns_zones.test", "zones.#", "2"),

					resource.TestCheckResourceAttrWith(
						"data.technitium_dns_zones.test",
						"zones.0.last_modified",
						func(value string) error {
							_, err := time.Parse(time.RFC3339Nano, value)
							if err != nil {
								return fmt.Errorf("last_modified is not a valid date: %s", value)
							}
							return nil
						},
					),
				),
			},
		},
	})
}
