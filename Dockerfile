# syntax=docker/dockerfile:1

# --- Frontend: build the Svelte app. Runs on the build machine's own
# architecture, because its output is plain files that every target shares.
FROM --platform=$BUILDPLATFORM node:26-alpine AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# Vite writes to ../internal/webui/dist, which is /src/internal/webui/dist here.
RUN npm run build && npm run size

# --- Backend: cross-compile the Go binary for the target architecture.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o /out/huginn ./cmd/huginn
# An empty /data owned by the non-root user, so a fresh volume is writable.
RUN mkdir /out/data

# --- Runtime: no shell, no package manager, runs as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/huginn /huginn
COPY --from=build --chown=65532:65532 /out/data /data

ENV HUGINN_ADDR=:8080 \
    HUGINN_DATA_DIR=/data
VOLUME /data
EXPOSE 8080
USER 65532:65532

# The image has no curl, so the binary checks itself.
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/huginn", "healthcheck"]

ENTRYPOINT ["/huginn"]
