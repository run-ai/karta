// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

// Exit codes. The numbers are not a contract; only that conditions differ.
const (
	ExitError    = 1 // cluster unreachable, auth failure, workload not found
	ExitUsage    = 2 // invalid flag value or argument combination
	ExitNotFound = 4 // no Karta definition covers the requested type
)

// exitError carries the exit code a failure should produce. A plain error still
// means ExitError.
type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string { return e.err.Error() }

func (e exitError) Unwrap() error { return e.err }

// ExitCode reports the process exit code; main matches the method, not the type.
func (e exitError) ExitCode() int { return e.code }
