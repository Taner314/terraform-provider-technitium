resource "technitium_dns_record_mx" "primary" {
  zone       = "example.com"
  name       = "@"
  preference = 10
  exchange   = "mail.example.com"
}
