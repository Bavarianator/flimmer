#!/usr/bin/env bash
# Packt ein Release-Ziel nach dist/ (web/dist muss gebaut sein). Genutzt von der CI und von release-local.sh.
#   scripts/package.sh linux amd64        scripts/package.sh linux arm 7        scripts/package.sh windows amd64
# Version und TMDB-Projekt-Key kommen aus VERSION bzw. TMDB_KEY.
set -euo pipefail
goos=$1 goarch=$2 goarm=${3:-}
VERSION=${VERSION:-dev}
cd "$(dirname "$0")/.."
export CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch GOARM=$goarm
ldflags="-s -w -X github.com/flimmer-media/flimmer/internal/meta.DefaultTMDBKey=${TMDB_KEY:-} -X github.com/flimmer-media/flimmer/internal/update.Version=$VERSION"
name=flimmer-$VERSION-$goos-$goarch${goarm:+v$goarm}
ext=; [ "$goos" = windows ] && ext=.exe
rm -rf "pkg/$name" && mkdir -p "pkg/$name" dist
go build -trimpath -o "pkg/$name/flimmer$ext" -ldflags "$ldflags" ./cmd/server
cp LICENSE "pkg/$name/"
if [ "$goos" = windows ]; then
  cp deploy/LIESMICH.txt "pkg/$name/"
  (cd pkg && rm -f "../dist/$name.zip" && zip -qr "../dist/$name.zip" "$name")
else
  tar -C pkg -czf "dist/$name.tar.gz" "$name"
fi
# Rendezvous-Dienst nur für Linux-Server als eigenes Asset
if [ "$goos" = linux ]; then
  relay=flimmer-relay-$VERSION-$goos-$goarch${goarm:+v$goarm}
  rm -rf "pkg/$relay" && mkdir -p "pkg/$relay"
  go build -trimpath -o "pkg/$relay/flimmer-relay" -ldflags "-s -w" ./cmd/relay
  cp LICENSE deploy/relay.service "pkg/$relay/"
  tar -C pkg -czf "dist/$relay.tar.gz" "$relay"
fi
rm -rf pkg
echo "dist/$name fertig"
