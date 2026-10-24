data "technitium_cluster" "this" {}

output "unreachable_nodes" {
  value = [for n in data.technitium_cluster.this.nodes : n.name if n.state == "Unreachable"]
}
