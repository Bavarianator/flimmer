#!/bin/sh
# Baut das webOS-Paket (.ipk). Braucht die webOS-CLI: npm i -g @webos-tools/cli
# Installieren auf dem TV (Developer Mode): ares-install -d <tv> dist/io.flimmer.app_*.ipk
set -e
cd "$(dirname "$0")"
rm -rf build && mkdir -p build dist
cp appinfo.json icon.png largeIcon.png build/
cp ../launcher/index.html build/
ares-package build -o dist
