terraform {
  required_providers {
    technitium = {
      source = "taner314/technitium"
    }
  }
}

provider "technitium" {
  host  = "https://dns.example.com:53443"
  token = var.technitium_token

  # Or log in with a username and password instead of a token:
  # username = "terraform"
  # password = var.technitium_password
}
