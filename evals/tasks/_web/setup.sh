#!/usr/bin/env bash
# Idempotent: install the shared web toolchain once (network needed only here).
set -euo pipefail
cd "$(dirname "$0")"
if [ ! -d node_modules/react ] || [ ! -d node_modules/playwright ]; then
  npm install --no-audit --no-fund
fi
npx --no-install playwright install chromium
