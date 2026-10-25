# DNS-over-TLS/HTTPS need a certificate on the server.
resource "technitium_settings_protocols" "this" {
  enable_dns_over_tls      = true
  enable_dns_over_https    = true
  tls_certificate_path     = "/etc/dns/certs/dns.example.com.pfx"
  tls_certificate_password = var.dns_certificate_password
}
