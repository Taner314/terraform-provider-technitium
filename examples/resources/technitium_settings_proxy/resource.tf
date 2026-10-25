# Outbound proxy for recursion, forwarding, block lists and app downloads.
resource "technitium_settings_proxy" "this" {
  type    = "Socks5"
  address = "10.0.0.5"
  port    = 1080
  bypass  = ["127.0.0.0/8", "10.0.0.0/8", "192.168.0.0/16", "localhost"]
}
