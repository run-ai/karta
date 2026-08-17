// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// This tool lives in its own module so its registry client (go-containerregistry)
// never enters Karta's shipped dependency graph or its generated license list.
module github.com/run-ai/karta/hack/imagelock

go 1.26.3

require (
	github.com/google/go-containerregistry v0.20.6
	sigs.k8s.io/yaml v1.6.0
)

require (
	github.com/containerd/stargz-snapshotter/estargz v0.16.3 // indirect
	github.com/docker/cli v28.2.2+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.3 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/vbatts/tar-split v0.12.1 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/sync v0.15.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
)
