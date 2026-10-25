# DS for a signed child zone. With the child hosted on Technitium too, take
# the values from technitium_dns_zone_dnssec.ds_records.
resource "technitium_dns_record_ds" "sub" {
  zone        = "example.com"
  name        = "sub"
  key_tag     = 12345
  algorithm   = "ECDSAP256SHA256"
  digest_type = "SHA256"
  digest      = "BBE017B17E5CB5FFFF1EC2C7815367DF80D8E7EAEE4832D3ED192159D79B1EEB"
}
