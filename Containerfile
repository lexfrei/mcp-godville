FROM docker.io/library/golang:1.27-alpine AS builder

ARG VERSION=development
ARG REVISION=unknown

# UPX is used for image-size win on a scratch image. Trade-offs noted:
# - small LZMA decompression cost on cold start (a few ms for a Go binary)
# - some AV/EDR tools flag packed ELFs as suspicious
# - won't work in sandboxes that block mprotect
# Drop UPX if any of these become a problem; -ldflags="-s -w" already
# strips symbols and DWARF, so the binary stays small without it.
# hadolint ignore=DL3018
RUN echo 'nobody:x:65534:65534:Nobody:/:' > /tmp/passwd && \
    apk add --no-cache upx ca-certificates

WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.revision=${REVISION}" \
    -trimpath ./cmd/mcp-godville && \
    upx --best --lzma mcp-godville

FROM scratch

COPY --from=builder /tmp/passwd /etc/passwd
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder --chmod=555 /build/mcp-godville /mcp-godville

USER 65534
ENTRYPOINT ["/mcp-godville"]
