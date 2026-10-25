# Let one user edit a single zone (e.g. for external-dns or ACME DNS challenges).
resource "technitium_user" "external_dns" {
  username = "external-dns"
  password = var.external_dns_password
}

resource "technitium_zone_permission" "external_dns" {
  zone       = "k8s.example.com"
  user       = technitium_user.external_dns.username
  can_view   = true
  can_modify = true
}
