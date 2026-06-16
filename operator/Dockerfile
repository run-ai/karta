# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation

# Build from the repository root so the `replace github.com/run-ai/karta => ../`
# directive in operator/go.mod resolves against the local pkg/ tree.
# Keep GO_VERSION aligned with the `go` directive in operator/go.mod:
#
#   docker build \
#     --build-arg GO_VERSION=$(awk '/^go /{print $2}' operator/go.mod) \
#     -f operator/Dockerfile -t karta-operator:latest .
ARG GO_VERSION=1.26.3
FROM golang:${GO_VERSION} AS build

WORKDIR /src
COPY . .
WORKDIR /src/operator
RUN CGO_ENABLED=0 GO111MODULE=on go build -trimpath -o /karta-operator ./cmd

FROM gcr.io/distroless/static:nonroot
COPY --from=build /karta-operator /karta-operator
USER 65532:65532
ENTRYPOINT ["/karta-operator"]
