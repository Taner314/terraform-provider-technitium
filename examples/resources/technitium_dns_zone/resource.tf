resource "technitium_dns_zone" "example" {
  name                       = "example.com"
  type                       = "Primary"
  use_soa_serial_date_scheme = true

  zone_transfer = "Deny"
  update        = "Deny"
}

resource "technitium_dns_zone" "reverse" {
  name = "1.168.192.in-addr.arpa"
  type = "Primary"
}

# Conditional forwarder zone; forwarders are managed as FWD records.
resource "technitium_dns_zone" "corp" {
  name = "corp.internal"
  type = "Forwarder"
}
