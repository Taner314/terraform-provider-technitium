resource "technitium_group" "dns_ops" {
  name        = "dns-ops"
  description = "Can edit zones"
}

resource "technitium_permission" "dns_ops_zones" {
  section    = "Zones"
  group      = technitium_group.dns_ops.name
  can_view   = true
  can_modify = true
}
