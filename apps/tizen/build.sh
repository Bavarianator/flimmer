#!/bin/sh
# Baut das Tizen-Paket (.wgt). Braucht Tizen Studio (CLI „tizen“) und ein Samsung-Zertifikatsprofil:
#   tizen certificate … && tizen security-profiles add -n flimmer …
# Installieren auf dem TV (Developer Mode): sdb connect <tv> && tizen install -n dist/Flimmer.wgt
set -e
cd "$(dirname "$0")"
PROFILE=${TIZEN_PROFILE:-flimmer}
rm -rf build && mkdir -p build dist
cp config.xml icon.png build/
cp ../launcher/index.html build/
tizen package -t wgt -s "$PROFILE" -- build
mv build/*.wgt dist/Flimmer.wgt
