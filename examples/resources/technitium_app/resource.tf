resource "technitium_app" "split" {
  name = "Split Horizon"
  config = jsonencode({
    appPreference            = 40
    enableAddressTranslation = false
    networks                 = { lan = ["192.168.0.0/16"] }
    domainGroupMap           = {}
    networkGroupMap          = {}
    groups                   = []
  })
}

resource "technitium_dns_record_app" "www" {
  zone        = "example.com"
  name        = "www"
  app_name    = technitium_app.split.name
  class_path  = "SplitHorizon.SimpleAddress"
  record_data = jsonencode({ public = ["203.0.113.10"], private = ["192.168.1.10"] })
}
