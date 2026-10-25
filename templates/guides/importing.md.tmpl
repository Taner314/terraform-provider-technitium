---
page_title: "Importing an existing server"
subcategory: "Guides"
---

# Importing an existing server

Every resource supports import, so an existing server can be brought under management without
recreating anything. With Terraform 1.5+ or OpenTofu, `import` blocks plus
`-generate-config-out` write the configuration for you.

## Import IDs

| Resource | ID format | Example |
|---|---|---|
| `technitium_dns_zone` | `<zone>` | `example.com` |
| `technitium_dns_record_*` | `<zone>/<name>[/<record data>]` | `example.com/www/192.168.1.20` |
| `technitium_dns_zone_dnssec` | `<zone>[/<algorithm>[/<curve>]]` | `example.com/ECDSA/P256` |
| `technitium_user`, `technitium_group` | name | `alice` |
| `technitium_permission` | `<section>/user\|group/<name>` | `Zones/group/dns-ops` |
| `technitium_zone_permission` | `<zone>/user\|group/<name>` | `example.com/user/alice` |
| `technitium_tsig_key` | key name | `xfr.example.com` |
| `technitium_blocked_zone`, `technitium_allowed_zone` | domain | `ads.example.net` |
| `technitium_app` | app name | `Split Horizon` |
| `technitium_dhcp_scope` | scope name | `lan` |
| `technitium_dhcp_reserved_lease` | `<scope>/<mac>` | `lan/00:11:22:33:44:55` |
| `technitium_settings_*`, `technitium_sso` | `settings` / `sso` | `settings` |

Record data is the type's identifying values in zone-file order, separated by spaces: an MX is
`<preference> <exchange>`, an SRV `<priority> <weight> <port> <target>`. Types with one record per
name (CNAME, DNAME, APP) use just `<zone>/<name>`. Each record resource's page lists its format.

## Example: a zone and its records

Use the `technitium_dns_records` data source to see what's there:

```terraform
data "technitium_dns_records" "all" {
  zone = "example.com"
}

output "records" {
  value = [for r in data.technitium_dns_records.all.records : "${r.type} ${r.name} ${jsonencode(r.data)}"]
}
```

Then import:

```terraform
import {
  to = technitium_dns_zone.example
  id = "example.com"
}

import {
  to = technitium_dns_record_a.www
  id = "example.com/www/192.168.1.20"
}

import {
  to = technitium_dns_record_mx.mail
  id = "example.com/@/10 mail.example.com"
}
```

```shell
terraform plan -generate-config-out=imported.tf
```

Review `imported.tf`, move the resources into your configuration and run `terraform plan` until it
shows no changes.

## Things to know

- Passwords and secrets (users, proxy, certificate passwords, SSO client secret) can't be read
  back. After importing, the first apply sets them to the configured value.
- Zone signing parameters can't be read back exactly: give the algorithm (and curve) in the
  import ID of `technitium_dns_zone_dnssec`.
- API tokens can't be imported, as the server only returns the secret when creating it.
