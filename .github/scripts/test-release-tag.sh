#!/bin/sh
set -eu

# The release plan requires a tag to match the actual pill version.
pill=$1
output=$("$pill" version --format toon)
version=$(printf '%s\n' "$output" | sed -n 's/^version: //p')
test -n "$version"
sh .github/scripts/check-release-tag.sh "v$version" "$pill"
if sh .github/scripts/check-release-tag.sh "v$version-mismatch" "$pill"; then
  echo 'A mismatched release tag was accepted' >&2
  exit 1
fi
