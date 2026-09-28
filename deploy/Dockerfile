# Multi-Arch-Image, Build aus dem Repo-Root:
#   docker buildx build -f deploy/Dockerfile --platform linux/amd64,linux/arm64,linux/arm/v7 -t flimmer .
# UI und Binary werden auf der Build-Plattform gebaut (Go cross-kompiliert), nur die Runtime-Stufe läuft pro Zielarchitektur.

FROM --platform=$BUILDPLATFORM node:22-slim AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27 AS server
ARG TARGETOS TARGETARCH TARGETVARIANT
# Projekt-Key für TMDB (optional); FLIMMER_TMDB_KEY zur Laufzeit überschreibt ihn.
ARG TMDB_KEY=""
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -trimpath -o /flimmer -ldflags "-s -w \
      -X github.com/flimmer-media/flimmer/internal/meta.DefaultTMDBKey=$TMDB_KEY \
      -X github.com/flimmer-media/flimmer/internal/update.Version=$VERSION" ./cmd/server

# Rendezvous-Dienst (optional, ohne Video-Traffic):
#   docker buildx build -f deploy/Dockerfile --target relay -t flimmer-relay .
FROM --platform=$BUILDPLATFORM golang:1.27 AS relay-build
ARG TARGETOS TARGETARCH TARGETVARIANT
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN mkdir /relaydata && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -trimpath -o /relay -ldflags "-s -w" ./cmd/relay

FROM gcr.io/distroless/static-debian12:nonroot AS relay
COPY --from=relay-build /relay /usr/local/bin/flimmer-relay
COPY --from=relay-build --chown=65532:65532 /relaydata /data
VOLUME ["/data"]
EXPOSE 8098
ENTRYPOINT ["/usr/local/bin/flimmer-relay", "-addr", ":8098", "-db", "/data/relay.db"]

# Standard-Image (letzte Stufe = Default-Target)
FROM debian:stable-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=server /flimmer /usr/local/bin/flimmer
VOLUME ["/media", "/data"]
EXPOSE 8096
ENTRYPOINT ["/usr/local/bin/flimmer"]
CMD ["-media", "/media", "-data", "/data"]
