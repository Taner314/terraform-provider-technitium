# Answer everything under legacy.example.com from example.net.
resource "technitium_dns_record_dname" "legacy" {
  zone  = "example.com"
  name  = "legacy"
  dname = "example.net"
}
