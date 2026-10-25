resource "technitium_settings_logging" "this" {
  logging_type       = "Console"
  log_queries        = false
  max_log_file_days  = 30
  max_stat_file_days = 365
}
