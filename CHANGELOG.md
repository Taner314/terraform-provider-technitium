# Changelog

All notable changes to this provider are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[semantic versioning](https://semver.org/). While the version is 0.x, minor
releases may contain breaking changes; they are listed under **Breaking**.

## [0.1.0] - 2026-10-25

First release of `taner314/technitium`. It started as a fork of
[kenske/technitium](https://registry.terraform.io/providers/kenske/technitium)
and was rewritten so that every resource is idempotent (a second `plan` is
empty), detects changes made outside Terraform, and can be imported.

Tested on Technitium DNS Server **14.3** and **15.6** with Terraform and OpenTofu.

### Added

- **Zones**: `technitium_dns_zone` for all zone types, with zone options
  (query access, zone transfer, notify, dynamic updates, catalog membership)
  updated in place. `technitium_dns_zone_dnssec` signs zones (NSEC/NSEC3) and
  exposes the DS records to publish at the parent. `technitium_tsig_key`.
- **Records**: one resource per type: `technitium_dns_record_{a, aaaa, ns,
  cname, ptr, mx, txt, rp, srv, naptr, dname, ds, sshfp, tlsa, svcb, https,
  uri, caa, aname, fwd, app}`. Records are identified by their data, so
  several records of one type at one name (round-robin A, multiple TXT) are
  managed independently; spelling the server normalises (case, trailing dots,
  IPv6, hex) does not cause diffs.
- **Blocking**: `technitium_blocked_zone`, `technitium_allowed_zone`; block
  list URLs in `technitium_settings_blocking`.
- **DNS apps**: `technitium_app` installs apps from the DNS App Store or a URL
  and manages their config.
- **Access control**: `technitium_user`, `technitium_group`,
  `technitium_permission`, `technitium_zone_permission`,
  `technitium_api_token`, `technitium_sso` (Technitium 15+).
- **Server settings**: `technitium_settings_{general, rate_limiting,
  web_service, protocols, recursion, cache, blocking, proxy, forwarders,
  logging}`. Each owns only the attributes you set; settings a server
  version doesn't have read as null and error clearly if set.
- **DHCP**: `technitium_dhcp_scope` with all scope options,
  `technitium_dhcp_reserved_lease`.
- **Data sources**: `technitium_dns_zone`, `technitium_dns_zones`,
  `technitium_dns_records`, `technitium_dhcp_scope`, `technitium_dhcp_scopes`,
  `technitium_dhcp_leases`, `technitium_server_info`, `technitium_cluster`.
- **Provider**: `skip_tls_verify`; token or username/password, all
  configurable through environment variables.
- **Docs**: guides for getting started, importing an existing server,
  Technitium 14 vs 15, and migrating from kenske/technitium.

### Breaking (compared to kenske/technitium)

- `technitium_dns_zone_record` is replaced by the per-type record resources,
  and `domain` (FQDN) by `name` relative to `zone` (`@` for the apex).
- `technitium_dhcp_reserved_lease.name` is renamed to `scope`.
- `technitium_dhcp_scope` options are authoritative: options removed from
  the configuration are cleared on the server.
- The `technitium_dns_zone_record` and `technitium_dns_zone_records` data
  sources are replaced by `technitium_dns_records`.

See the [migration guide](https://registry.terraform.io/providers/taner314/technitium/latest/docs/guides/migrating-from-kenske).

### Fixed (compared to kenske/technitium)

- Works with Technitium 15: login, the `Authorization` header and empty
  request parameters (which v15 treats as explicit values).
- `technitium_dns_zone` now reads and updates; previously drift was never
  detected.
- DHCP: DNS servers were sent with empty entries, reserved leases were never
  read back, and removed exclusions were not cleared.
- Parallel changes to records of the same name and type no longer lose
  records (Technitium 14 rewrites the whole RRset on each change).
- The client retries while the server restarts its web service, and never
  puts the API token in URLs or logs.

[0.1.0]: https://github.com/Taner314/terraform-provider-technitium/releases/tag/v0.1.0
