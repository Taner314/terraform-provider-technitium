# Terraform Provider - Technitium DNS Server

Terraform/OpenTofu provider for the Technitium DNS Server [API](https://github.com/TechnitiumSoftware/DnsServer/blob/master/APIDOCS.md).
Tested against Technitium v14.x and v15.x.

Forked from [kenske/terraform-provider-technitium](https://github.com/kenske/terraform-provider-technitium) (MIT).
Not yet published to a public registry; see [Provider Development](#provider-development) for local use.

> **0.x, breaking changes expected.** Resource schemas differ from upstream: the single
> `technitium_dns_zone_record` resource was replaced by one resource per record type.

## Getting Started

Add the provider to your Terraform configuration:

```hcl
terraform {
  required_providers {
    technitium = {
      version = "~> 0.2.2"
      source = "registry.terraform.io/stellivix/technitium"
    }
  }
}

```

Then, configure the provider with the host and token:

```hcl
provider "technitium" {
  host  = var.TECHNITIUM_HOST
  token = var.TECHNITIUM_TOKEN
}
```

Alternatively, the values can be set using the `TECHNITIUM_HOST` and 
`TECHNITIUM_TOKEN` environment variables.

Complete example:

```hcl
terraform {
  required_providers {
    technitium = {
      source = "registry.terraform.io/stellivix/technitium"
    }
  }
}

provider "technitium" {
  host  = var.TECHNITIUM_HOST
  token = var.TECHNITIUM_TOKEN
}

resource "technitium_dns_zone" "example" {
  name = "example.com"
  type = "Primary"
}

resource "technitium_dns_record_a" "www" {
  zone       = technitium_dns_zone.example.name
  name       = "www"
  ip_address = "192.168.1.10"
  ttl        = 300
}

resource "technitium_dns_record_cname" "docs" {
  zone  = technitium_dns_zone.example.name
  name  = "docs"
  cname = "www.example.com"
}

resource "technitium_dns_record_mx" "mail" {
  zone       = technitium_dns_zone.example.name
  name       = "@"
  preference = 10
  exchange   = "mail.example.com"
}
```

Each record type has its own resource (`technitium_dns_record_<type>`). Records
are identified by zone, name and their type-specific data, so several records
of one type at one name (round-robin A, multiple TXT) are managed independently.
Every resource supports `terraform import`; see the docs for the ID format.

## Provider Development

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:
1. Setup the override in your `~/.terraformrc` file as described in the Terraform provider [documentation](https://developer.hashicorp.com/terraform/tutorials/providers-plugin-framework/providers-plugin-framework-provider#prepare-terraform-for-local-provider-install):
```
provider_installation {

  dev_overrides {
      "registry.terraform.io/stellivix/technitium" = "/path/to/go/bin" # e.g. /home/user/go/bin
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}

```
5. Use `TF_LOG_PROVIDER=debug` to enable debug logging for the provider.

### Tests

```sh
docker compose -f docker/docker-compose.yaml up -d   # Technitium on localhost:5380
make test                                            # unit tests
TECHNITIUM_HOST=http://localhost:5380 make testacc   # acceptance tests
```

CI (`.gitlab-ci.yml`) runs the acceptance suite against Technitium 14.x and 15.x with both Terraform and OpenTofu.

