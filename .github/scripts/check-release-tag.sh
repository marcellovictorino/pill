#!/bin/sh
set -eu

tag=$1
pill=$2
output=$("$pill" version --format toon)
version=$(printf '%s\n' "$output" | sed -n 's/^version: //p')
if [ -z "$version" ] || [ "$tag" != "v$version" ]; then
  printf 'Tag %s does not match pill version %s\n' "$tag" "${version:-unknown}" >&2
  exit 1
fi
