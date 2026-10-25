resource "technitium_dns_record_caa" "le" {
  zone  = "example.com"
  name  = "@"
  flags = 0
  tag   = "issue"
  value = "letsencrypt.org"
}

resource "technitium_dns_record_caa" "iodef" {
  zone  = "example.com"
  name  = "@"
  flags = 0
  tag   = "iodef"
  value = "mailto:security@example.com"
}
