resource "technitium_dns_record_txt" "spf" {
  zone = "example.com"
  name = "@"
  text = "v=spf1 mx -all"
}
