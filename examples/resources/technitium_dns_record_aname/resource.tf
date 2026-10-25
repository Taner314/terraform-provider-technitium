# CNAME-like aliasing at the zone apex: the server answers with the target's addresses.
resource "technitium_dns_record_aname" "apex" {
  zone  = "example.com"
  name  = "@"
  aname = "example.gitlab.io"
}
