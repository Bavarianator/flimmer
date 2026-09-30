#!/bin/sh
# Flimmer installieren oder aktualisieren – ein Befehl, nur Docker nötig:
#   curl -fsSL https://raw.githubusercontent.com/Bavarianator/flimmer/main/deploy/install.sh | sh -s -- /pfad/zu/filmen
# Ohne Argument wird ./medien verwendet. Nochmal ausführen = aktualisieren, die Daten bleiben im Volume flimmer-data.
# Optional: FLIMMER_PORT (Standard 8096, z. B. wenn Jellyfin dort läuft), FLIMMER_IMAGE (anderes Image), TZ.
set -eu
media=${1:-$PWD/medien}
port=${FLIMMER_PORT:-8096}
image=${FLIMMER_IMAGE:-ghcr.io/bavarianator/flimmer:edge}

command -v docker >/dev/null || { echo "Docker fehlt: https://docs.docker.com/get-docker/" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "Docker läuft nicht oder braucht Rechte: Docker starten bzw. mit sudo ausführen." >&2; exit 1; }
mkdir -p "$media"
media=$(cd "$media" && pwd)

# Fertiges Image von GitHub; klappt das nicht, aus dem Quellcode bauen (dauert ein paar Minuten).
if ! docker pull -q "$image" >/dev/null; then
  echo "Image nicht erreichbar, baue Flimmer selbst …"
  image=flimmer:local
  docker build -q -f deploy/Dockerfile -t "$image" https://github.com/Bavarianator/flimmer.git#main >/dev/null
fi

set -- --restart unless-stopped -v "$media:/media:ro" -v flimmer-data:/data -e "TZ=${TZ:-Europe/Berlin}"
# Linux: Host-Netz, damit Apps und Fernseher den Server im Heimnetz von selbst finden. Docker Desktop: Port freigeben.
if [ "$(uname -s)" = Linux ]; then set -- "$@" --network host; else set -- "$@" -p "$port:$port"; fi
if [ -e /dev/dri ]; then set -- "$@" --device /dev/dri:/dev/dri; fi # Hardware-Transcoding (Intel/AMD)

docker rm -f flimmer >/dev/null 2>&1 || true
docker run -d --name flimmer "$@" "$image" -media /media -data /data -addr ":$port" >/dev/null

ip=$(ip -4 route get 1.1.1.1 2>/dev/null | sed -n 's/.* src \([0-9.]*\).*/\1/p') || true
echo "Flimmer läuft: http://${ip:-localhost}:$port  (Medien: $media)"
echo "Im Browser öffnen und das Admin-Konto anlegen. Aktualisieren: diesen Befehl einfach noch einmal ausführen."
