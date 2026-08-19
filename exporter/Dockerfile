# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation

# Build from the repository root so the `replace github.com/run-ai/karta => ../`
# directive in exporter/go.mod resolves against the local pkg/ tree.
# Keep GO_VERSION aligned with the `go` directive in exporter/go.mod.
ARG GO_VERSION=1.26.3
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=0.0.0-main

WORKDIR /src
COPY . .
WORKDIR /src/exporter
RUN CGO_ENABLED=0 GO111MODULE=on GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
	go build -trimpath \
	-ldflags "-X github.com/run-ai/karta/pkg/version.version=${VERSION}" \
	-o /karta-exporter ./cmd

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /karta-exporter /karta-exporter
USER 65532:65532
ENTRYPOINT ["/karta-exporter"]
