resource "technitium_user" "external_dns" {
  username     = "external-dns"
  password     = var.external_dns_password
  display_name = "external-dns (k8s)"
  groups       = [technitium_group.dns_ops.name]
}
