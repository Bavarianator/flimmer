#!/bin/sh
# Flimmer per Docker installieren – ein Befehl im Terminal:
#   curl -fsSL https://raw.githubusercontent.com/Bavarianator/flimmer/main/deploy/install.sh | sh -s -- /pfad/zu/filmen
# Ohne Argument wird ./medien verwendet. Nochmal ausführen = aktualisieren (Daten bleiben im Volume).
set -eu
media=${1:-$PWD/medien}
command -v docker >/dev/null || { echo "Docker fehlt: https://docs.docker.com/get-docker/" >&2; exit 1; }
mkdir -p "$media"
docker build -q -f deploy/Dockerfile -t flimmer https://github.com/Bavarianator/flimmer.git#main
docker rm -f flimmer >/dev/null 2>&1 || true
docker run -d --name flimmer --restart unless-stopped -p 8096:8096 \
  -v "$(cd "$media" && pwd):/media:ro" -v flimmer-data:/data flimmer >/dev/null
echo "Flimmer läuft: http://localhost:8096 (Medien: $media)"
