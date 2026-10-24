resource "technitium_dns_record_cname" "docs" {
  zone  = "example.com"
  name  = "docs"
  cname = "example.gitlab.io"
}
