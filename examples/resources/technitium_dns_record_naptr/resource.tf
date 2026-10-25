resource "technitium_dns_record_naptr" "sip" {
  zone        = "example.com"
  name        = "@"
  order       = 100
  preference  = 10
  flags       = "U"
  services    = "E2U+sip"
  regexp      = "!^.*$!sip:info@example.com!"
  replacement = "."
}
