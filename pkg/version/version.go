// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package version exposes the build information stamped into karta binaries at
// link time via -ldflags "-X github.com/run-ai/karta/pkg/version.<var>=<value>".
// Both the CLI and the operator import this package so they can never report
// different versions.
package version

import (
	"fmt"
	"runtime"
)

// Populated at link time. The defaults apply to a plain `go build`.
var (
	version   = "0.0.0-dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// Info is the build information reported by karta binaries.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

func Get() Info {
	return Info{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

func String() string { return version }
