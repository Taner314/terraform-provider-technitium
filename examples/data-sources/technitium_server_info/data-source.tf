data "technitium_server_info" "this" {}

output "technitium_version" {
  value = data.technitium_server_info.this.version
}
