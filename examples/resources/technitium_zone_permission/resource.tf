resource "technitium_zone_permission" "external_dns" {
  zone       = "k8s.example.com"
  user       = technitium_user.external_dns.username
  can_view   = true
  can_modify = true
}
