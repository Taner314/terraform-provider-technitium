# Only the attributes you set are managed; the rest follow the server.
resource "technitium_settings_general" "this" {
  dns_server_domain          = "dns.example.com"
  default_record_ttl         = 3600
  default_responsible_person = "hostmaster@example.com"
  dnssec_validation          = true
  ipv6_mode                  = "Enabled"
}
