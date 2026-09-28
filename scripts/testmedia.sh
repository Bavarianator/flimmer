#!/usr/bin/env bash
# Erzeugt kurze Testclips (320x180, 10–60 s), die jeden Wiedergabe-Pfad abdecken.
# Aufruf: scripts/testmedia.sh [zielordner]   (Standard: testdata/media)
# ponytail: kein PGS-Clip, ffmpeg hat keinen PGS-Encoder; bei Bedarf eine echte .sup-Probe einbinden.
set -euo pipefail

out=${1:-testdata/media}
mkdir -p "$out/Testserie/Staffel 1"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

printf '1\n00:00:01,000 --> 00:00:04,000\nHallo Welt\n\n2\n00:00:05,000 --> 00:00:08,000\nZweite Zeile\n' > "$tmp/de.srt"
cat > "$tmp/en.ass" <<'EOF'
[Script Info]
ScriptType: v4.00+

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, Bold, Italic, Alignment, MarginL, MarginR, MarginV
Style: Default,Arial,20,&H00FFFFFF,0,0,2,10,10,10

[Events]
Format: Layer, Start, End, Style, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,Hello world
EOF

ff() { ffmpeg -hide_banner -loglevel error -y -f lavfi -i testsrc2=size=320x180:rate=25:duration=${D:-10} -f lavfi -i sine=frequency=440:duration=${D:-10} "$@"; }
h264=(-c:v libx264 -preset ultrafast -g 50 -pix_fmt yuv420p)

# Direct Play: MP4 mit H.264 + AAC
ff "${h264[@]}" -c:a aac -movflags +faststart "$out/Direkt (2020).mp4"
# Direct Stream: MKV (Container passt nicht) + Text-Untertitel SRT und ASS; 60 s für den Seek-Test
D=60 ff -i "$tmp/de.srt" -i "$tmp/en.ass" -map 0 -map 1 -map 2 -map 3 "${h264[@]}" -c:a aac -c:s:0 srt -c:s:1 ass \
  -metadata:s:s:0 language=ger -metadata:s:s:1 language=eng "$out/Remux (2021).mkv"
# Nur Ton transkodieren: DTS 5.1
ff "${h264[@]}" -c:a dca -strict -2 -ac 6 "$out/Tonwandel (2022).mkv"
# Voll transkodieren: HEVC 10 bit
ff -c:v libx265 -preset ultrafast -pix_fmt yuv420p10le -x265-params log-level=error:keyint=50 -c:a aac "$out/Umwandeln (2023).mkv"
# Episode, Serienname kommt aus dem Ordner
ff "${h264[@]}" -c:a aac "$out/Testserie/Staffel 1/S01E01.mp4"

echo "Testmedien in $out"
