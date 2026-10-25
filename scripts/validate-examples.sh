#!/usr/bin/env bash
# Validates every example against the provider's schema: builds the
# provider, points Terraform at it with dev_overrides and runs
# `terraform validate` per example directory (with a stub provider block
# and string variables for any var.* the example uses).
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

go build -o "$work/bin/terraform-provider-technitium" "$root"
cat > "$work/tfrc" <<EOT
provider_installation {
  dev_overrides { "registry.terraform.io/taner314/technitium" = "$work/bin" }
  direct {}
}
EOT

failed=0
for dir in "$root"/examples/provider "$root"/examples/resources/* "$root"/examples/data-sources/*; do
  w="$work/$(basename "$dir")"
  mkdir -p "$w"
  cp "$dir"/*.tf "$w/"
  sed -i 's#source *= *"taner314/technitium"#source = "registry.terraform.io/taner314/technitium"#' "$w"/*.tf
  if ! grep -q 'provider "technitium"' "$w"/*.tf; then
    cat > "$w/_provider.tf" <<'EOT'
terraform {
  required_providers {
    technitium = { source = "registry.terraform.io/taner314/technitium" }
  }
}
provider "technitium" {
  host  = "http://localhost:5380"
  token = "x"
}
EOT
  fi
  for v in $(grep -ohE 'var\.[a-zA-Z0-9_]+' "$w"/*.tf | sort -u | cut -d. -f2); do
    grep -q "variable \"$v\"" "$w"/*.tf || printf 'variable "%s" {\n  type    = string\n  default = "x"\n}\n' "$v" >> "$w/_stub_vars.tf"
  done
  if ! out=$(cd "$w" && TF_CLI_CONFIG_FILE="$work/tfrc" terraform validate -no-color 2>&1); then
    echo "FAIL ${dir#"$root"/}"
    echo "$out" | sed 's/^/    /'
    failed=1
  fi
done

[ "$failed" = 0 ] && echo "All examples are valid."
exit "$failed"
