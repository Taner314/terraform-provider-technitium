resource "technitium_dhcp_reserved_lease" "nas" {
  scope            = technitium_dhcp_scope.lan.name
  hardware_address = "00:11:22:33:44:55"
  ip_address       = "192.168.1.10"
  host_name        = "nas"
  comments         = "Storage"
}
