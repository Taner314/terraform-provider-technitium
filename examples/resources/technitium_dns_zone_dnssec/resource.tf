resource "technitium_dns_zone_dnssec" "example" {
  zone      = "example.com"
  algorithm = "ECDSA"
  curve     = "P256"
  nx_proof  = "NSEC3"
}

# Publish at the parent, e.g. with your registrar's provider.
output "ds" {
  value = [for d in technitium_dns_zone_dnssec.example.ds_records : d if d.digest_type == "SHA256"]
}
