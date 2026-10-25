data "technitium_dhcp_leases" "lan" {
  scope = "lan"
}

output "clients" {
  value = { for l in data.technitium_dhcp_leases.lan.leases : l.host_name => l.address if l.host_name != "" }
}
