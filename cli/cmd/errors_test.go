// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"errors"
	"io"
	"testing"

	"github.com/spf13/cobra"
)

// exitCodeOf runs the root command with args and reports the code the binary
// would exit with.
func exitCodeOf(t *testing.T, args ...string) (int, error) {
	t.Helper()

	root := NewRootCommand()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs(args)

	err := root.Execute()
	if err == nil {
		return 0, nil
	}
	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		return coded.ExitCode(), err
	}
	return ExitError, err
}

// An unrecognised command is a usage error, not a cluster failure.
func TestUnknownCommandIsAUsageError(t *testing.T) {
	code, err := exitCodeOf(t, "wrkload")
	if err == nil {
		t.Fatal("expected an error for an unknown command")
	}
	if code != ExitUsage {
		t.Errorf("expected exit %d, got %d (%v)", ExitUsage, code, err)
	}
}

// An invalid flag value anywhere in the tree is a usage error, without the
// command having to wrap it.
func TestInvalidFlagValueIsAUsageError(t *testing.T) {
	root := NewRootCommand()
	root.AddCommand(&cobra.Command{Use: "noop", RunE: func(*cobra.Command, []string) error { return nil }})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"-o", "bogus", "noop"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error for an invalid output format")
	}
	var coded interface{ ExitCode() int }
	if !errors.As(err, &coded) || coded.ExitCode() != ExitUsage {
		t.Errorf("expected exit %d, got %v", ExitUsage, err)
	}
}

// A command that rejects its arguments is a usage error too.
func TestArgumentErrorIsAUsageError(t *testing.T) {
	code, err := exitCodeOf(t, "definition", "bogus")
	if err == nil {
		t.Fatal("expected an error for an unexpected argument")
	}
	if code != ExitUsage {
		t.Errorf("expected exit %d, got %d (%v)", ExitUsage, code, err)
	}
}

// The root prints help rather than failing when invoked bare.
func TestRootWithoutArgsSucceeds(t *testing.T) {
	if code, err := exitCodeOf(t); err != nil || code != 0 {
		t.Errorf("expected a clean exit, got %d (%v)", code, err)
	}
}
