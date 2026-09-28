#!/usr/bin/env bash
# E2E-Smoke-Test: baut den Server, erzeugt Testmedien, startet ihn und prüft jeden Wiedergabe-Pfad per HTTP.
# Aufruf aus dem Repo-Root: scripts/smoke.sh   (braucht go, ffmpeg, curl, python3)
set -euo pipefail

# Freier Port, damit parallele Läufe (andere Sessions, CI-Matrix) sich nicht gegenseitig testen.
port=${PORT:-$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')}
base=http://127.0.0.1:$port
work=$(mktemp -d)
trap 'kill $pid 2>/dev/null; wait $pid 2>/dev/null; rm -rf "$work"' EXIT
pid=

test -f web/dist/index.html || { mkdir -p web/dist && echo '<!doctype html><title>Flimmer</title>' > web/dist/index.html; }
go build -o "$work/flimmer" ./cmd/server
"$(dirname "$0")/testmedia.sh" "$work/media"
"$work/flimmer" -addr "127.0.0.1:$port" -media "$work/media" -data "$work/data" > "$work/server.log" 2>&1 &
pid=$!

for _ in $(seq 60); do kill -0 $pid 2>/dev/null && curl -sf "$base/" >/dev/null && break; sleep 1; done
kill -0 $pid 2>/dev/null && curl -sf "$base/" >/dev/null || { cat "$work/server.log"; echo "FEHLER: Server startet nicht"; exit 1; }

# Seit der Anmeldung braucht die API eine Sitzung: Admin per Einrichtung anlegen, Cookie für alle Aufrufe.
curl() { command curl -b "$work/jar" -c "$work/jar" "$@"; }
curl -sf -X POST -d "{\"name\":\"Smoke\",\"password\":\"smoke123\",\"dirs\":[\"$work/media\"]}" "$base/api/setup" >/dev/null ||
  { cat "$work/server.log"; echo "FEHLER: Einrichtung fehlgeschlagen"; exit 1; }

fail=0
ok()   { echo "  ok   $*"; }
bad()  { echo "  FAIL $*"; fail=1; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }

profile='{"containers":["mp4"],"video":["h264"],"audio":["aac"],"nativeHls":false,"maxBitrate":0}'

# Titel → erwartete Methode
declare -A want=(
  ["Direkt"]=direct-play
  ["Remux"]=direct-stream
  ["Tonwandel"]=transcode-audio
  ["Umwandeln"]=transcode
  ["Episode 1"]=direct-play
  ["HDR"]=transcode
)
# Der Scan läuft im Hintergrund: warten, bis alle Titel da sind.
for _ in $(seq 120); do
  lib=$(curl -sf -X POST -d "$profile" "$base/api/library")
  n=$(json 'len(d or [])' <<<"$lib")
  [ "$n" -ge ${#want[@]} ] && break
  sleep 1
done
lib=$(json 'json.dumps(d or [])' <<<"$lib")
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
    # Mitte zuerst (= Spulen, ffmpeg startet neu), dann Anfang und Ende. Jedes Segment muss abspielbar sein
    # und so lang wie in der Playlist angekündigt – sonst stockt der Player nach dem Spulen.
    mapfile -t extinf < <(grep -o '^#EXTINF:[0-9.]*' <<<"$pl" | cut -d: -f2)
    for s in $((segs / 2)) 0 $((segs - 1)); do
      codecs= dur=
      if curl -sf -o "$work/seg.ts" "${base}${url%index.m3u8}$s.ts" &&
         codecs=$(ffprobe -v error -show_entries stream=codec_name -of csv=p=0 "$work/seg.ts" | sort -u | tr '\n' ' ') &&
         grep -q h264 <<<"$codecs" && grep -q aac <<<"$codecs"; then
        dur=$(ffprobe -v error -select_streams v -show_entries packet=pts_time -of csv=p=0 "$work/seg.ts" | sort -n | sed -n '1p;$p' | paste -sd' ' | awk '{printf "%.2f", $2-$1+0.04}')
        python3 -c "import sys; d,w=map(float,sys.argv[1:]); sys.exit(abs(d-w)>0.6 or d>7)" "$dur" "${extinf[$s]}" &&
          ok "Segment $s: $codecs${dur}s (Playlist ${extinf[$s]}s)" || bad "Segment $s dauert ${dur}s, Playlist sagt ${extinf[$s]}s"
        # HDR-Quelle im SDR-Profil: Segmente müssen tone-gemappt (BT.709, 8 bit) sein, sonst sieht man Grauschleier
        if [[ $url == */h264*-sdr/* ]]; then
          trc=$(ffprobe -v error -select_streams v -show_entries stream=color_transfer,pix_fmt -of csv=p=0 "$work/seg.ts" | sed -n 1p)
          [ "$trc" = "yuv420p,bt709" ] && ok "Segment $s tone-gemappt ($trc)" || bad "Segment $s nicht tone-gemappt ($trc)"
        fi
      else
        bad "Segment $s nicht abspielbar (${codecs:-leer})"
      fi
    done
  fi

  for sub in $(json '" ".join(str(s["index"]) for s in d.get("subtitles") or [] if s["format"]=="vtt")' <<<"$play"); do
    curl -sf "$base/api/items/$id/subs/$sub.vtt" | sed -n 1p | grep -q WEBVTT && ok "Untertitel $sub → WebVTT" || bad "Untertitel $sub"
  done
done < <(json '"\n".join(i["id"]+"\t"+i["title"] for i in d)' <<<"$lib")

if [ $fail != 0 ]; then echo; echo "--- server.log ---"; tail -50 "$work/server.log"; exit 1; fi
echo "Alles grün."
