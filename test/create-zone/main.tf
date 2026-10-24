# Manual playground: `make install`, set dev_overrides (see README), then
# `docker compose -f docker/docker-compose.yaml up -d` and `terraform apply`.
terraform {
  required_providers {
    technitium = {
      source = "stellivix/technitium"
    }
  }
}

provider "technitium" {
  host     = "http://localhost:5380"
  username = "admin"
  password = "password"
}

resource "technitium_dns_zone" "example" {
  name = "example3.com"
  type = "Primary"
}

resource "technitium_dns_record_cname" "cname" {
  zone     = technitium_dns_zone.example.name
  name     = "cname"
  cname    = "other.example3.com"
  comments = "This is a test CNAME record"
  ttl      = 420
}

resource "technitium_dns_record_a" "a" {
  zone       = technitium_dns_zone.example.name
  name       = "testa"
  ip_address = "1.1.1.1"
  comments   = "This is a test A record"
  ttl        = 420
}

resource "technitium_dns_zone" "forwarder" {
  name = "fwd.example3.com"
  type = "Forwarder"
}

resource "technitium_dns_record_fwd" "forwarder" {
  zone              = technitium_dns_zone.forwarder.name
  name              = "@"
  forwarder         = "this-server"
  protocol          = "Https"
  dnssec_validation = true
}
