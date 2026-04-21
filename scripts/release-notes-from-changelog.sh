#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 <version> [repo-slug]" >&2
  exit 1
fi

version="$1"
repo_slug="${2:-}"
changelog_path="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/CHANGELOG.md"

section="$(
  awk -v version="$version" '
    $0 ~ "^## \\[" version "\\] - " { in_section=1 }
    in_section {
      if ($0 ~ "^## \\[" && $0 !~ "^## \\[" version "\\] - ") {
        exit
      }
      print
    }
  ' "$changelog_path"
)"

if [[ -z "$section" ]]; then
  echo "could not find changelog section for version $version" >&2
  exit 1
fi

printf '%s\n' "$section"

if [[ -n "$repo_slug" ]]; then
  previous_version="$(
    awk -v version="$version" '
      $0 ~ "^## \\[" version "\\] - " { found=1; next }
      found && $0 ~ /^## \[[^]]+\] - / {
        line = $0
        sub(/^## \[/, "", line)
        sub(/\].*/, "", line)
        print line
        exit
      }
    ' "$changelog_path"
  )"

  if [[ -n "$previous_version" && "$previous_version" != "Unreleased" ]]; then
    printf '\n**Full Changelog**: https://github.com/%s/compare/v%s...v%s\n' \
      "$repo_slug" "$previous_version" "$version"
  fi
fi
