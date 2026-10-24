resource "technitium_sso" "this" {
  enabled           = true
  authority         = "https://auth.example.com/application/o/technitium/"
  client_id         = "technitium"
  client_secret     = var.oidc_client_secret
  additional_scopes = ["email", "groups"]
  group_map = [
    { remote_group = "dns-admins", local_group = "DNS Administrators" },
  ]
}
