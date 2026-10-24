provider "technitium" {
  alias    = "dns2"
  host     = "http://192.168.1.3:5380"
  username = "admin"
  password = var.dns2_admin_password
}

resource "technitium_cluster_secondary" "dns2" {
  provider                  = technitium.dns2
  primary_node_url          = technitium_cluster.this.nodes[0].url
  primary_node_ip_address   = "192.168.1.2"
  primary_node_username     = "admin"
  primary_node_password     = var.dns1_admin_password
  ignore_certificate_errors = true
  ip_addresses              = ["192.168.1.3"]
}
