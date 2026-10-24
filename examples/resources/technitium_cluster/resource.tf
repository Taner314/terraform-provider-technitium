resource "technitium_cluster" "this" {
  cluster_domain = "dns.example.com"
  ip_addresses   = ["192.168.1.2"]
}
