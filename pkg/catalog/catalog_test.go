// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package catalog

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog/kartas"
)

// TestGetHit resolves a known workload GVK to its built-in Karta.
func TestGetHit(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}
	k, err := Get(gvk)
	if err != nil {
		t.Fatalf("Get(%s): %v", gvk, err)
	}
	if got := k.Spec.StructureDefinition.RootComponent.Kind; got == nil || got.Kind != "Job" {
		t.Fatalf("resolved root kind = %v, want Job", got)
	}
}

// TestGetMiss returns ErrNotFound for an unregistered GVK.
func TestGetMiss(t *testing.T) {
	_, err := Get(schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Nope"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get miss error = %v, want ErrNotFound", err)
	}
}

// TestGetReturnsDeepCopy asserts that mutating a Karta returned by Get does not
// affect the catalog: a later Get returns the unmodified definition.
func TestGetReturnsDeepCopy(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}
	first, err := Get(gvk)
	if err != nil {
		t.Fatalf("Get(%s): %v", gvk, err)
	}
	original := first.Spec.StructureDefinition.RootComponent.Name
	first.Spec.StructureDefinition.RootComponent.Name = original + "-mutated"

	second, err := Get(gvk)
	if err != nil {
		t.Fatalf("Get(%s): %v", gvk, err)
	}
	if got := second.Spec.StructureDefinition.RootComponent.Name; got != original {
		t.Fatalf("catalog mutated through Get: root name = %q, want %q", got, original)
	}
}

// TestListDeterministic asserts List is sorted by GVK and stable across calls.
func TestListDeterministic(t *testing.T) {
	first := List()
	second := List()
	if len(first) != len(second) {
		t.Fatalf("List length changed: %d vs %d", len(first), len(second))
	}
	for i := range first {
		gi := RootKey(first[i])
		gj := RootKey(second[i])
		if gi != gj {
			t.Fatalf("List order not stable at %d: %s vs %s", i, gi, gj)
		}
		if i > 0 {
			prev := RootKey(first[i-1])
			if prev.String() > gi.String() {
				t.Fatalf("List not sorted: %s before %s", prev, gi)
			}
		}
	}
}

// TestListReturnsCopy asserts mutating the returned slice does not affect the
// catalog's internal order.
func TestListReturnsCopy(t *testing.T) {
	l := List()
	if len(l) < 2 {
		t.Skip("need at least two definitions")
	}
	l[0], l[1] = l[1], l[0]
	again := List()
	if again[0] == l[0] {
		t.Fatal("List did not return a defensive copy")
	}
}

// TestListReturnsDeepCopies asserts mutating a Karta returned by List does not
// affect the catalog.
func TestListReturnsDeepCopies(t *testing.T) {
	l := List()
	if len(l) == 0 {
		t.Skip("catalog is empty")
	}
	original := l[0].Spec.StructureDefinition.RootComponent.Name
	l[0].Spec.StructureDefinition.RootComponent.Name = original + "-mutated"

	again := List()
	if got := again[0].Spec.StructureDefinition.RootComponent.Name; got != original {
		t.Fatalf("catalog mutated through List: root name = %q, want %q", got, original)
	}
}

// TestNewPanicsOnDuplicateGVK asserts two definitions with the same root GVK
// fail loudly at construction.
func TestNewPanicsOnDuplicateGVK(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate GVK")
		}
	}()
	newCatalog([]func() *v1alpha1.Karta{kartas.BatchJob, kartas.BatchJob})
}

// TestNewPanicsOnIncompleteGVK asserts a definition whose root GVK is missing a
// version or kind fails loudly at construction rather than being indexed under a
// partial GVK. An empty group stays valid (core workloads such as Pod).
func TestNewPanicsOnIncompleteGVK(t *testing.T) {
	withKind := func(gvk *v1alpha1.GroupVersionKind) func() *v1alpha1.Karta {
		return func() *v1alpha1.Karta {
			return &v1alpha1.Karta{Spec: v1alpha1.KartaSpec{StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{Name: "root", Kind: gvk},
			}}}
		}
	}
	cases := map[string]*v1alpha1.GroupVersionKind{
		"missing version": {Group: "example.com", Kind: "Thing"},
		"missing kind":    {Group: "example.com", Version: "v1"},
	}
	for name, gvk := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic on incomplete root GVK")
				}
			}()
			newCatalog([]func() *v1alpha1.Karta{withKind(gvk)})
		})
	}
}

// TestRoundTrip asserts each definition marshals byte-for-byte to its committed
// docs/catalog file. This gives the git diff --exit-code drift guard at the
// unit-test level.
func TestRoundTrip(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	catalogDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "docs", "catalog")

	for _, k := range List() {
		t.Run(k.Name, func(t *testing.T) {
			slug, err := Slug(k)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(catalogDir, slug+".yaml"))
			if err != nil {
				t.Fatalf("read committed catalog file: %v (run `make generate-samples`)", err)
			}
			got, err := MarshalYAML(k)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("generated YAML for %s differs from docs/catalog/%s.yaml; run `make generate-samples`", k.Name, slug)
			}
		})
	}
}

// TestCatalogFilesUnmarshalAndValidate reads each committed docs/catalog file,
// unmarshals it back into a Karta, and runs the shared validator. This guards the
// on-disk YAML directly: it proves the generated files parse (including folded
// multi-line expressions) and satisfy the same validation as the Go definitions.
func TestCatalogFilesUnmarshalAndValidate(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	catalogDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "docs", "catalog")

	files, err := filepath.Glob(filepath.Join(catalogDir, "*.yaml"))
	if err != nil {
		t.Fatalf("glob catalog dir: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no catalog files found")
	}

	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var k v1alpha1.Karta
			if err := yaml.Unmarshal(data, &k); err != nil {
				t.Fatalf("unmarshal %s: %v", path, err)
			}
			if err := v1alpha1.NewKartaValidator(&k).Validate(); err != nil {
				t.Errorf("validate %s: %v", path, err)
			}
		})
	}
}

// TestListEntriesAreValid asserts every built-in definition carries the Karta
// TypeMeta and passes the shared Karta validator.
func TestListEntriesAreValid(t *testing.T) {
	entries := List()
	if len(entries) == 0 {
		t.Fatal("builtin catalog is empty")
	}
	for _, k := range entries {
		t.Run(k.Name, func(t *testing.T) {
			if k.APIVersion != "run.ai/v1alpha1" {
				t.Errorf("apiVersion = %q, want run.ai/v1alpha1", k.APIVersion)
			}
			if k.Kind != "Karta" {
				t.Errorf("kind = %q, want Karta", k.Kind)
			}
			if err := v1alpha1.NewKartaValidator(k).Validate(); err != nil {
				t.Errorf("validate: %v", err)
			}
		})
	}
}
