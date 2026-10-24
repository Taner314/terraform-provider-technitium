resource "technitium_dhcp_scope" "lan" {
  name             = "lan"
  enabled          = true
  starting_address = "192.168.1.100"
  ending_address   = "192.168.1.250"
  subnet_mask      = "255.255.255.0"
  router_address   = "192.168.1.1"
  domain_name      = "lan"
  dns_servers      = ["192.168.1.2"]
  lease_time_days  = 7

  exclusions = [
    { starting_address = "192.168.1.200", ending_address = "192.168.1.210" },
  ]
  static_routes = [
    { destination = "10.8.0.0", subnet_mask = "255.255.0.0", router = "192.168.1.3" },
  ]
}
