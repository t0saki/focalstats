# syntax=docker/dockerfile:1
#
# Multi-arch build via Go cross-compilation. The build stage always runs on the
# native BUILDPLATFORM and cross-compiles for TARGETOS/TARGETARCH, so building
# linux/amd64 + linux/arm64 needs no QEMU emulation. The final stage is scratch
# (no RUN instructions), so the resulting image is just the static binary.

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-s -w" -trimpath -o /out/focalstats .

FROM scratch
COPY --from=build /out/focalstats /focalstats
ENTRYPOINT ["/focalstats"]
LABEL org.opencontainers.image.source="https://github.com/t0saki/focalstats" \
      org.opencontainers.image.description="Count focal-length usage from photo EXIF" \
      org.opencontainers.image.licenses="MIT"
