#!/usr/bin/env bash
# Baut ein Release lokal wie die CI – aus einem sauberen Worktree von REF (Standard HEAD) nach dist/.
#   VERSION=v0.1.0 scripts/release-local.sh
# Server für linux amd64/arm64/armv7, windows amd64, darwin arm64 (+ Relay für Linux), SHA256SUMS.
# webOS-IPK per ares-package (notfalls über npx), Tizen-WGT nur mit installierter Tizen-CLI. Veröffentlicht wird nichts.
set -euo pipefail
VERSION=${VERSION:-v0.1.0}
repo=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'cd "$repo"; git worktree remove --force "$work/src" 2>/dev/null; rm -rf "$work"; git worktree prune' EXIT
git -C "$repo" worktree add -q --detach "$work/src" "${REF:-HEAD}"
cd "$work/src"

# Vorhandene node_modules des Haupt-Repos nutzen (kein Netz nötig), sonst npm ci.
if [ -d "$repo/web/node_modules" ]; then ln -s "$repo/web/node_modules" web/node_modules; else (cd web && npm ci --silent --no-audit --no-fund); fi
(cd web && npm run build --silent)
export VERSION
for t in "linux amd64" "linux arm64" "linux arm 7" "windows amd64" "darwin arm64"; do
  scripts/package.sh $t
done

# ares-package ohne globale Installation: npx lädt @webos-tools/cli in den npm-Cache.
if ! command -v ares-package >/dev/null && command -v npx >/dev/null; then
  mkdir -p "$work/bin" && printf '#!/bin/sh\nexec npx -y -p @webos-tools/cli ares-package "$@"\n' > "$work/bin/ares-package"
  chmod +x "$work/bin/ares-package" && export PATH="$work/bin:$PATH"
fi
if command -v ares-package >/dev/null; then
  sh apps/webos/build.sh && cp apps/webos/dist/*.ipk dist/
else
  echo "webOS-IPK übersprungen (npm i -g @webos-tools/cli)"
fi
if command -v tizen >/dev/null; then
  sh apps/tizen/build.sh && cp apps/tizen/dist/Flimmer.wgt "dist/Flimmer-$VERSION.wgt"
else
  echo "Tizen-WGT übersprungen (Tizen Studio CLI fehlt)"
fi

(cd dist && sha256sum -- * > SHA256SUMS)
mkdir -p "$repo/dist"
cp dist/* "$repo/dist/"
echo "Release $VERSION ($(git rev-parse --short HEAD)) in $repo/dist:"
ls -la "$repo/dist"
