#!/bin/sh
# Development release script (Linux/macOS): only builds the Astro site.
set -e
repo="$(cd "$(dirname "$0")/../.." && pwd)"
echo "[release.dev] repo=$repo"
cd "$repo"
pnpm -C site build
echo "[release.dev] done"
