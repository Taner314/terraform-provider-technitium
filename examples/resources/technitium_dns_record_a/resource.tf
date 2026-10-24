resource "technitium_dns_record_a" "www" {
  zone       = "example.com"
  name       = "www"
  ip_address = "192.168.1.10"
  ttl        = 300
  ptr        = true # also create 10.1.168.192.in-addr.arpa
}

# Round-robin: one resource per address. Records of one type at one name
# share a TTL.
resource "technitium_dns_record_a" "lb" {
  for_each   = toset(["192.168.1.21", "192.168.1.22"])
  zone       = "example.com"
  name       = "lb"
  ip_address = each.value
}
