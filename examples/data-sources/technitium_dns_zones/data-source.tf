data "technitium_dns_zones" "all" {}

output "primary_zones" {
  value = [for z in data.technitium_dns_zones.all.zones : z.name if z.type == "Primary" && !z.internal]
}
