#!/bin/sh
# Prints the CHANGELOG.md section for a version (without the heading), e.g.
#   scripts/release-notes.sh 0.1.0
# Exits 1 if there is no such section.
set -eu
version=${1#v}
notes=$(awk -v v="$version" '
  /^## \[/ { if (found) exit; if (index($0, "## [" v "]") == 1) { found = 1; next } }
  /^\[[^]]+\]: / { if (found) exit }
  found { print }
  END { if (!found) exit 1 }
' "$(dirname "$0")/../CHANGELOG.md") || exit 1
printf '%s\n' "$notes" | sed -e '/./,$!d'
