#!/bin/sh
# Erzeugt die Probe-Clips (Schicht 3 der Geräteerkennung): je ≤1 s, 64x36, ein Merkmal pro Clip.
# Ergebnis wird eingecheckt (public/probe/, gesamt <300 KB), damit der Web-Build kein ffmpeg braucht.
set -e
cd "$(dirname "$0")/../public/probe"
rm -f ./*.mp4 ./*.mkv ./*.webm ./*.ts
V="-f lavfi -i testsrc2=size=64x36:rate=10:duration=1"
A="-f lavfi -i sine=frequency=440:duration=1"
A6="-f lavfi -i sine=frequency=440:duration=1,pan=5.1|c0=c0|c1=c0|c2=c0|c3=c0|c4=c0|c5=c0"
q="-hide_banner -loglevel error -y"
H264="-c:v libx264 -preset ultrafast -pix_fmt yuv420p -crf 40"
MP4="-movflags +faststart"

ffmpeg $q $V $H264 -an $MP4 h264.mp4
ffmpeg $q $V -c:v libx265 -x265-params log-level=error -crf 40 -pix_fmt yuv420p -tag:v hvc1 -an $MP4 hevc.mp4
ffmpeg $q $V -c:v libx265 -x265-params log-level=error -crf 40 -pix_fmt yuv420p10le -tag:v hvc1 -an $MP4 hevc10.mp4
ffmpeg $q $V -c:v libsvtav1 -preset 12 -crf 60 -pix_fmt yuv420p -an $MP4 av1.mp4 2>/dev/null
ffmpeg $q $V -c:v libvpx-vp9 -deadline realtime -crf 60 -b:v 0 -an vp9.webm
ffmpeg $q $V $H264 -an mkv.mkv
ffmpeg $q $V $H264 -an ts.ts
ffmpeg $q $V $A $H264 -c:a aac -b:a 32k $MP4 aac.mp4
ffmpeg $q $V $A6 $H264 -c:a ac3 -b:a 192k $MP4 ac3.mp4
ffmpeg $q $V $A6 $H264 -c:a eac3 -b:a 192k $MP4 eac3.mp4
ffmpeg $q $V $A $H264 -c:a mp3 -b:a 32k $MP4 mp3.mp4
ffmpeg $q $V $A $H264 -c:a libopus -b:a 16k $MP4 opus.mp4
ffmpeg $q $V $A $H264 -c:a flac -sample_fmt s16 -ar 8000 $MP4 flac.mp4
ffmpeg $q $V $A $H264 -c:a dca -strict -2 -b:a 192k dts.mkv
ffmpeg $q $V -f lavfi -i sine=frequency=440:duration=0.3:sample_rate=44100 $H264 -c:a truehd -strict -2 truehd.mkv
du -ch ./* | tail -1
