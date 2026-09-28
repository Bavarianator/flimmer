#!/usr/bin/env bash
# E2E-Smoke-Test: baut den Server, erzeugt Testmedien, startet ihn und prüft jeden Wiedergabe-Pfad per HTTP.
# Aufruf aus dem Repo-Root: scripts/smoke.sh   (braucht go, ffmpeg, curl, python3)
set -euo pipefail

port=${PORT:-18096}
base=http://127.0.0.1:$port
work=$(mktemp -d)
trap 'kill $pid 2>/dev/null; wait $pid 2>/dev/null; rm -rf "$work"' EXIT
pid=

test -f web/dist/index.html || { mkdir -p web/dist && echo '<!doctype html><title>Flimmer</title>' > web/dist/index.html; }
go build -o "$work/flimmer" ./cmd/server
"$(dirname "$0")/testmedia.sh" "$work/media"
"$work/flimmer" -addr "127.0.0.1:$port" -media "$work/media" -data "$work/data" > "$work/server.log" 2>&1 &
pid=$!

for _ in $(seq 60); do curl -sf "$base/" >/dev/null && break; sleep 1; done
curl -sf "$base/" >/dev/null || { cat "$work/server.log"; echo "FEHLER: Server startet nicht"; exit 1; }

fail=0
ok()   { echo "  ok   $*"; }
bad()  { echo "  FAIL $*"; fail=1; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }

profile='{"containers":["mp4"],"video":["h264"],"audio":["aac"],"nativeHls":false,"maxBitrate":0}'
lib=$(curl -sf -X POST -d "$profile" "$base/api/library")

# Titel → erwartete Methode
declare -A want=(
  ["Direkt"]=direct-play
  ["Remux"]=direct-stream
  ["Tonwandel"]=transcode-audio
  ["Umwandeln"]=transcode
  ["Episode 1"]=direct-play
)
n=$(json 'len(d)' <<<"$lib")
[ "$n" = ${#want[@]} ] && ok "$n Titel gefunden" || bad "$n Titel statt ${#want[@]}"

while IFS=$'\t' read -r id title; do
  echo "$title"
  play=$(curl -sf -X POST -d "$profile" "$base/api/items/$id/play")
  method=$(json 'd["method"]' <<<"$play")
  url=$(json 'd["url"]' <<<"$play")
  [ "$method" = "${want[$title]:-?}" ] && ok "Methode $method" || bad "Methode $method, erwartet ${want[$title]:-?}"

  if [ "$method" = direct-play ]; then
    code=$(curl -s -o "$work/range" -w '%{http_code}' -H 'Range: bytes=0-99' "$base$url")
    [ "$code" = 206 ] && [ "$(stat -c %s "$work/range")" = 100 ] && ok "Range → 206" || bad "Range → $code"
  else
    pl=$(curl -sf "$base$url")
    segs=$(grep -c '\.ts$' <<<"$pl" || true)
    grep -q '#EXT-X-ENDLIST' <<<"$pl" && [ "$segs" -gt 0 ] && ok "Playlist vollständig ($segs Segmente)" || bad "Playlist unvollständig"
    # erstes und letztes Segment (= Spulen ans Ende) müssen abspielbares MPEG-TS sein
    for s in 0 $((segs - 1)); do
      if curl -sf -o "$work/seg.ts" "${base}${url%index.m3u8}$s.ts" &&
         codecs=$(ffprobe -v error -show_entries stream=codec_name -of csv=p=0 "$work/seg.ts" | sort -u | tr '\n' ' ') &&
         grep -q h264 <<<"$codecs" && grep -q aac <<<"$codecs"; then
        ok "Segment $s: $codecs"
      else
        bad "Segment $s nicht abspielbar (${codecs:-leer})"
      fi
    done
  fi

  for sub in $(json '" ".join(str(s["index"]) for s in d.get("subtitles") or [] if s["format"]=="vtt")' <<<"$play"); do
    curl -sf "$base/api/items/$id/subs/$sub.vtt" | head -1 | grep -q WEBVTT && ok "Untertitel $sub → WebVTT" || bad "Untertitel $sub"
  done
done < <(json '"\n".join(i["id"]+"\t"+i["title"] for i in d)' <<<"$lib")

if [ $fail != 0 ]; then echo; echo "--- server.log ---"; tail -50 "$work/server.log"; exit 1; fi
echo "Alles grün."
