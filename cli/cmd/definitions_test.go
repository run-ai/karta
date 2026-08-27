// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import "testing"

func TestDefinitionsRejectsArgs(t *testing.T) {
	_, err := execute(t, "definitions", "bogus")
	if err == nil {
		t.Fatal("expected error for unknown argument, got nil")
	}
}
