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
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags "-s -w" -o /flimmer ./cmd/server

FROM debian:stable-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=server /flimmer /usr/local/bin/flimmer
VOLUME ["/media", "/data"]
EXPOSE 8096
ENTRYPOINT ["/usr/local/bin/flimmer"]
CMD ["-media", "/media", "-data", "/data"]
