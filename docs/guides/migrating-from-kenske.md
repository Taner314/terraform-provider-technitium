---
page_title: "Migrating from kenske/technitium"
subcategory: "Guides"
---

# Migrating from kenske/technitium

This provider started as a fork of [kenske/technitium](https://registry.terraform.io/providers/kenske/technitium)
and changed several schemas. Resources can't be moved between providers automatically, so the
simplest path is to remove the old resources from state and import them into the new ones —
nothing on the server is recreated.

## What changed

| kenske/technitium | taner314/technitium |
|---|---|
| `technitium_dns_zone_record` (all types, `domain` = FQDN) | one resource per type, e.g. `technitium_dns_record_a`, with `name` relative to `zone` (`@` for the apex) |
| `technitium_dns_zone` with every attribute forcing replacement | zone options are updated in place; unset options follow the server |
| `technitium_dhcp_reserved_lease.name` | renamed to `scope` |
| `technitium_dhcp_scope` (basic options) | all scope options; removed options are cleared |
| `technitium_dns_zone_record` / `_records` data sources | `technitium_dns_records` |

## Steps

1. Switch the provider source to `taner314/technitium` and run `terraform init -upgrade`.
2. Rewrite each `technitium_dns_zone_record` as the matching typed resource:

   ```terraform
   # before
   resource "technitium_dns_zone_record" "www" {
     zone       = "example.com"
     domain     = "www.example.com"
     type       = "A"
     ip_address = "192.168.1.20"
   }

   # after
   resource "technitium_dns_record_a" "www" {
     zone       = "example.com"
     name       = "www"
     ip_address = "192.168.1.20"
   }
   ```

3. Remove the old resources from state and import the new ones:

   ```shell
   terraform state rm technitium_dns_zone_record.www
   ```

   ```terraform
   import {
     to = technitium_dns_record_a.www
     id = "example.com/www/192.168.1.20"
   }
   ```

4. Run `terraform plan` until it shows no changes; see [Importing](importing) for every ID format.
