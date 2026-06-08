# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation

# Build from the repository root so the `replace github.com/run-ai/karta => ../../../`
# directive in go.mod resolves:
#   docker build -f docs/examples/controller-runtime/Dockerfile -t generic-controller-example:latest .
FROM golang:1.26.3 AS build
WORKDIR /src
COPY . .
WORKDIR /src/docs/examples/controller-runtime
RUN CGO_ENABLED=0 GO111MODULE=on go build -trimpath -o /controller .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /controller /controller
USER 65532:65532
ENTRYPOINT ["/controller"]
