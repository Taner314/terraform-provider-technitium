resource "technitium_dns_record_https" "apex" {
  zone           = "example.com"
  name           = "@"
  priority       = 1
  target_name    = "."
  params         = { alpn = "h2,h3" }
  auto_ipv4_hint = true
}
