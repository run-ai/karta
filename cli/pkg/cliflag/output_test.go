// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cliflag

import (
	"errors"
	"testing"
)

func TestOutputSetValid(t *testing.T) {
	o := NewOutput()
	for _, v := range o.Allowed() {
		if err := o.Set(v); err != nil {
			t.Errorf("Set(%q) returned error: %v", v, err)
		}
		if o.String() != v {
			t.Errorf("after Set(%q), String() = %q", v, o.String())
		}
		if string(o.Get()) != v {
			t.Errorf("after Set(%q), Get() = %q", v, o.Get())
		}
	}
}

func TestOutputSetInvalid(t *testing.T) {
	o := NewOutput()
	err := o.Set("bogus")
	if err == nil {
		t.Fatal("expected error for invalid output value, got nil")
	}
	if !errors.Is(err, ErrInvalidValue) {
		t.Errorf("error is not ErrInvalidValue: %v", err)
	}
	// An invalid Set must not mutate the value away from its default.
	if o.Get() != OutputTable {
		t.Errorf("value changed after failed Set: %q", o.String())
	}
}

func TestOutputType(t *testing.T) {
	if got := NewOutput().Type(); got != "output" {
		t.Errorf("Type() = %q, want %q", got, "output")
	}
}
