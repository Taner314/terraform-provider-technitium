# Changing ports or TLS changes how the provider must reach the server.
resource "technitium_settings_web_service" "this" {
  enable_tls              = true
  tls_port                = 53443
  http_to_tls_redirect    = true
  tls_certificate_path    = "/etc/dns/certs/dns.example.com.pfx"
  reverse_proxy_addresses = ["10.0.0.2"]
}
