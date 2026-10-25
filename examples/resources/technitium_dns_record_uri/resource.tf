resource "technitium_dns_record_uri" "ftp" {
  zone     = "example.com"
  name     = "_ftp._tcp"
  priority = 10
  weight   = 1
  uri      = "ftp://ftp.example.com/public"
}
