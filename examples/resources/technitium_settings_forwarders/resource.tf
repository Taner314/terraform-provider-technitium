resource "technitium_settings_forwarders" "this" {
  forwarders = ["dns.quad9.net (9.9.9.9:853)", "cloudflare-dns.com (1.1.1.1:853)"]
  protocol   = "Tls"
}
