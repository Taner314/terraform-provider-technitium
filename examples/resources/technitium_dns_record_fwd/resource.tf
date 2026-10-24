resource "technitium_dns_record_fwd" "corp" {
  zone              = "corp.internal"
  name              = "@"
  protocol          = "Tls"
  forwarder         = "dns.corp.example"
  dnssec_validation = true
}
