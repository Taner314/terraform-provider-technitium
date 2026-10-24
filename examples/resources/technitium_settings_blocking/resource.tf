resource "technitium_settings_blocking" "this" {
  enable_blocking = true
  blocking_type   = "NxDomain"
  block_list_urls = [
    "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts",
  ]
}
