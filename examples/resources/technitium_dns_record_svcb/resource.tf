# Advertise DNS-over-HTTPS for the server (RFC 9461).
resource "technitium_dns_record_svcb" "dns" {
  zone        = "example.com"
  name        = "_dns"
  priority    = 1
  target_name = "dns.example.com"
  params = {
    alpn = "h2,h3"
    port = "443"
  }
}
