#!/usr/bin/env bash
# Baut Flimmer für linux/amd64 und startet ihn auf einem Linux-Rechner per SSH – ohne root, alles unter ~/flimmer.
# Idempotent: erneut ausführen = neue Version deployen und neu starten.
#   scripts/deploy-nas.sh            deployen und starten
#   scripts/deploy-nas.sh stop       stoppen
#   scripts/deploy-nas.sh remove     stoppen und ~/flimmer auf dem Ziel löschen (nur das!)
# Bewusst kein „flimmer install“: das würde loginctl enable-linger setzen, eine Einstellung außerhalb von Flimmer.
set -euo pipefail

NAS=${NAS:-mo@192.168.55.190}
KEY=${KEY:-$HOME/.ssh/id_ed25519_flimmer_nas}
PORT=${PORT:-8097}
MEDIA=${MEDIA:-/media/jellyfin/filme,/media/jellyfin/serien}
DIR=flimmer # unter ~ auf dem Ziel
FFMPEG_URL=https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz
CACHE=${XDG_CACHE_HOME:-$HOME/.cache}/flimmer-deploy

remote() { ssh -i "$KEY" -o IdentitiesOnly=yes -o BatchMode=yes "$NAS" "$@"; }
stop='pid=$(cat ~/'$DIR'/flimmer.pid 2>/dev/null) && kill "$pid" 2>/dev/null && while kill -0 "$pid" 2>/dev/null; do sleep 0.2; done; rm -f ~/'$DIR'/flimmer.pid'

case ${1:-deploy} in
stop) remote "$stop; true"; echo "gestoppt"; exit ;;
remove) remote "$stop; rm -rf ~/$DIR"; echo "~/$DIR auf $NAS entfernt"; exit ;;
esac

# Gebaut wird der committete Stand (REF, Standard HEAD) in einem eigenen Worktree –
# halbfertige Änderungen im Arbeitsbaum (auch anderer Sessions) landen so nie auf dem Ziel.
repo=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'cd "$repo"; git worktree remove --force "$work/src" 2>/dev/null; rm -rf "$work"; git worktree prune' EXIT
git -C "$repo" worktree add -q --detach "$work/src" "${REF:-HEAD}"
version=$(git -C "$work/src" describe --tags --always)
# PATCH=datei: eigene, noch nicht committete Änderungen obendrauf (git diff -- <eigene Dateien> > datei).
[ -z "${PATCH:-}" ] || { git -C "$work/src" apply "$(realpath "$PATCH")"; version+=-patch; }
cd "$work/src"
# Vorhandene node_modules des Haupt-Repos nutzen (kein Netz nötig), sonst npm ci.
if [ -d "$repo/web/node_modules" ]; then ln -s "$repo/web/node_modules" web/node_modules; else (cd web && npm ci --silent --no-audit --no-fund); fi
(cd web && npm run build --silent)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "$work/flimmer" \
  -ldflags "-s -w -X github.com/Bavarianator/flimmer/internal/update.Version=$version" ./cmd/server

# Statisches ffmpeg einmal lokal holen (mit Prüfsumme) und nur ffmpeg/ffprobe übertragen.
if ! remote "test -x ~/$DIR/ffmpeg && test -x ~/$DIR/ffprobe"; then
  mkdir -p "$CACHE"
  if [ ! -x "$CACHE/ffprobe" ]; then
    curl -fsSL -o "$CACHE/ff.tar.xz" "$FFMPEG_URL"
    want=$(curl -fsSL "${FFMPEG_URL%/*}/checksums.sha256" | awk -v f="${FFMPEG_URL##*/}" '$2 == f {print $1}')
    echo "$want  $CACHE/ff.tar.xz" | sha256sum -c --quiet
    tar -xJf "$CACHE/ff.tar.xz" -C "$CACHE" --strip-components=2 --wildcards '*/bin/ffmpeg' '*/bin/ffprobe'
    rm "$CACHE/ff.tar.xz"
  fi
  remote "mkdir -p ~/$DIR"
  scp -q -i "$KEY" -o IdentitiesOnly=yes "$CACHE/ffmpeg" "$CACHE/ffprobe" "$NAS:$DIR/"
fi

# VAAPI ohne root: Fehlt dem System libva bzw. ein Intel-Treiber, holt apt-get download (als Benutzer) die
# Ubuntu/Debian-Pakete und entpackt sie nach ~/flimmer/va. i965-va-driver-shaders enthält die Encoder-Shader
# (ohne sie bricht h264_vaapi auf Braswell/Broadwell mit einer Assertion ab), iHD deckt neuere Intel-GPUs ab.
[ -n "${NO_VA:-}" ] || remote 'test -e /dev/dri/renderD128 && command -v apt-get >/dev/null &&
  ! ls /usr/lib/x86_64-linux-gnu/dri/*_drv_video.so 2>/dev/null | grep -qE "iHD|i965" && [ ! -d ~/'$DIR'/va/root ] || exit 0
  mkdir -p ~/'$DIR'/va/deb && cd ~/'$DIR'/va/deb &&
  for p in libva2 libva-drm2 i965-va-driver-shaders intel-media-va-driver-non-free; do apt-get download -q "$p" >/dev/null 2>&1 || echo "VA: $p nicht verfügbar"; done
  for d in *.deb; do dpkg-deb -x "$d" ../root; done'

remote "mkdir -p ~/$DIR/data"
scp -q -i "$KEY" -o IdentitiesOnly=yes "$work/flimmer" "$NAS:$DIR/flimmer.new"
# ffmpeg liegt neben der Binary → der Server findet es ohne PATH-Änderung.
# Jeder SSH-Login bekommt die aktuellen Gruppen (z. B. render/video für VAAPI) – ein Neustart übernimmt sie.
remote "$stop; cd ~/$DIR && mv flimmer.new flimmer && chmod +x flimmer ffmpeg ffprobe
  va=\$HOME/$DIR/va/root/usr/lib/x86_64-linux-gnu
  [ -d \$va ] && export LD_LIBRARY_PATH=\$va LIBVA_DRIVERS_PATH=\$va/dri
  nohup setsid ./flimmer -addr :$PORT -data ~/$DIR/data -media '$MEDIA' >> flimmer.log 2>&1 < /dev/null &
  echo \$! > flimmer.pid"

host=${NAS#*@}
for _ in $(seq 30); do
  if curl -fsS -o /dev/null "http://$host:$PORT/"; then
    echo "Flimmer $version läuft: http://$host:$PORT  (Log: ssh … tail -f ~/$DIR/flimmer.log)"
    exit 0
  fi
  sleep 1
done
remote "tail -30 ~/$DIR/flimmer.log"
echo "FEHLER: Server antwortet nicht auf Port $PORT" >&2
exit 1
