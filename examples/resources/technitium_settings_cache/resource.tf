resource "technitium_settings_cache" "this" {
  serve_stale        = true
  maximum_entries    = 50000
  minimum_record_ttl = 10
  maximum_record_ttl = 86400
  enable_prefetch    = true
}
