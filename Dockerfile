FROM node:20 AS webui-builder

WORKDIR /app/webui
COPY webui/package.json webui/package-lock.json ./
RUN npm ci
COPY webui ./
RUN npm run build

FROM golang:1.24 AS go-builder
WORKDIR /app
ARG TARGETOS=linux
ARG TARGETARCH=amd64
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/ds2api ./cmd/ds2api

FROM debian:bookworm-slim AS runtime-base
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget && rm -rf /var/lib/apt/lists/*

FROM runtime-base AS runtime-from-source
COPY --from=go-builder /out/ds2api /usr/local/bin/ds2api
COPY --from=go-builder /app/sha3_wasm_bg.7b9ca65ddd.wasm /app/sha3_wasm_bg.7b9ca65ddd.wasm
COPY --from=go-builder /app/config.example.json /app/config.example.json
COPY --from=webui-builder /app/static/admin /app/static/admin

FROM runtime-base AS runtime-from-dist
ARG TARGETARCH
COPY dist/docker-input/linux_amd64.tar.gz /tmp/ds2api_linux_amd64.tar.gz
COPY dist/docker-input/linux_arm64.tar.gz /tmp/ds2api_linux_arm64.tar.gz
RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) ARCHIVE="/tmp/ds2api_linux_amd64.tar.gz" ;; \
      arm64) ARCHIVE="/tmp/ds2api_linux_arm64.tar.gz" ;; \
      *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    tar -xzf "${ARCHIVE}" -C /tmp; \
    PKG_DIR="$(find /tmp -maxdepth 1 -type d -name "ds2api_*_linux_${TARGETARCH}" | head -n1)"; \
    test -n "${PKG_DIR}"; \
    install -Dm755 "${PKG_DIR}/ds2api" /usr/local/bin/ds2api; \
    install -Dm644 "${PKG_DIR}/sha3_wasm_bg.7b9ca65ddd.wasm" /app/sha3_wasm_bg.7b9ca65ddd.wasm; \
    install -Dm644 "${PKG_DIR}/config.example.json" /app/config.example.json; \
    mkdir -p /app/static; \
    cp -R "${PKG_DIR}/static/admin" /app/static/admin; \
    rm -rf /tmp/ds2api_* /tmp/ds2api_linux_amd64.tar.gz /tmp/ds2api_linux_arm64.tar.gz

EXPOSE 5001
CMD ["/usr/local/bin/ds2api"]

FROM runtime-from-source AS final
