# The token is stored in state; protect the state.
resource "technitium_api_token" "external_dns" {
  user = technitium_user.external_dns.username
  name = "external-dns"
}

output "external_dns_token" {
  value     = technitium_api_token.external_dns.token
  sensitive = true
}
