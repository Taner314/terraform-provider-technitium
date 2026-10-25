resource "technitium_dns_record_rp" "apex" {
  zone       = "example.com"
  name       = "@"
  mailbox    = "hostmaster@example.com"
  txt_domain = "contact.example.com"
}
