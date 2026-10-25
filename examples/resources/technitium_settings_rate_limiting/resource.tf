resource "technitium_settings_rate_limiting" "this" {
  ipv4_prefix_limits = [
    { prefix = 32, udp_limit = 600, tcp_limit = 600 },
    { prefix = 24, udp_limit = 6000, tcp_limit = 6000 },
  ]
  bypass_list = ["192.168.0.0/16"]
}
