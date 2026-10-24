resource "technitium_dns_record_srv" "sip" {
  zone     = "example.com"
  name     = "_sip._tcp"
  priority = 10
  weight   = 5
  port     = 5060
  target   = "sip.example.com"
}
