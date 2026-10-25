---
page_title: "Getting started"
subcategory: "Guides"
---

# Getting started

This walks from an empty Technitium server to a managed zone with a few records.

## 1. Create an API user and token

In the web console, create a user (e.g. `terraform`), add it to the *DNS Administrators* group
(or grant narrower permissions), then create an API token for it under *Administration →
Sessions*. Export the connection details:

```shell
export TECHNITIUM_HOST=https://dns.example.com:53443
export TECHNITIUM_TOKEN=...
```

## 2. Configure the provider

```terraform
terraform {
  required_providers {
    technitium = {
      source  = "taner314/technitium"
    }
  }
}

provider "technitium" {}
```

## 3. Add a zone and records

```terraform
resource "technitium_dns_zone" "home" {
  name = "home.example.com"
  type = "Primary"
}

resource "technitium_dns_record_a" "www" {
  zone       = technitium_dns_zone.home.name
  name       = "www"
  ip_address = "192.168.1.20"
  ttl        = 300
}

resource "technitium_dns_record_cname" "docs" {
  zone  = technitium_dns_zone.home.name
  name  = "docs"
  cname = "www.home.example.com"
}

resource "technitium_dns_record_txt" "spf" {
  zone = technitium_dns_zone.home.name
  name = "@"
  text = "v=spf1 -all"
}
```

`name` is relative to the zone; use `@` for the zone apex.

## 4. Apply

```shell
terraform init
terraform apply
```

Running `terraform plan` again should show no changes. If a record is changed or deleted in the
web console, the next plan shows the difference and `apply` restores it.

## Next steps

- Manage server-wide settings with the `technitium_settings_*` resources.
- Bring existing zones under management: [Importing an existing server](importing).
