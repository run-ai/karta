// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flagtypes

import "testing"

func TestOutputSetValid(t *testing.T) {
	for _, v := range OutputValues {
		o := NewOutput()
		if err := o.Set(v); err != nil {
			t.Errorf("Set(%q) returned error: %v", v, err)
		}
		if o.String() != v {
			t.Errorf("after Set(%q), String() = %q", v, o.String())
		}
	}
}

func TestOutputSetInvalid(t *testing.T) {
	o := NewOutput()
	if err := o.Set("bogus"); err == nil {
		t.Fatal("expected error for invalid output value, got nil")
	}
	// An invalid Set must not mutate the value away from its default.
	if o.String() != string(OutputTable) {
		t.Errorf("value changed after failed Set: %q", o.String())
	}
}

func TestOutputType(t *testing.T) {
	if got := NewOutput().Type(); got != "output" {
		t.Errorf("Type() = %q, want %q", got, "output")
	}
}
