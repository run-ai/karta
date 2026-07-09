# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation

# Shared tool versions, included by both the root Makefile and operator/Makefile
# so there is a single source of truth. Override on the command line if needed,
# e.g. make lint GOLANGCI_LINT_VERSION=v2.13.0
GOLANGCI_LINT_VERSION ?= v2.12.2
