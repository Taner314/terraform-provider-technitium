resource "technitium_blocked_zone" "ads" {
  for_each = toset(["ads.example.com", "tracker.example.net"])
  domain   = each.value
}
