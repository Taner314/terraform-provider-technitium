resource "technitium_settings_recursion" "this" {
  recursion   = "UseSpecifiedNetworkACL"
  network_acl = ["192.168.0.0/16", "10.0.0.0/8"]
}
