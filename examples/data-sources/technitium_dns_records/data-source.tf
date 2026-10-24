data "technitium_dns_records" "mx" {
  zone = "example.com"
  type = "MX"
}
