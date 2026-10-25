resource "technitium_dns_record_aaaa" "www" {
  zone       = "example.com"
  name       = "www"
  ip_address = "2001:db8::10"
  ttl        = 300
}
