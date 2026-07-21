// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package catalog exposes the immutable catalog of built-in Karta definitions,
// keyed by the root component's workload GVK. The catalog is built once from the
// typed Go definitions in pkg/catalog/kartas and never mutated, so callers can
// resolve a built-in Karta for a workload GVK with no cluster access.
package catalog

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog/kartas"
)

// yamlHeader is prepended to every generated catalog file.
const yamlHeader = "# SPDX-License-Identifier: Apache-2.0\n# Copyright (c) 2026 NVIDIA Corporation\n"

// ErrNotFound is returned by Get when no built-in Karta is registered for a GVK.
var ErrNotFound = fmt.Errorf("catalog: no Karta registered for GVK")

// definitions is the single wiring point for the catalog supported workloads.
// Adding a workload means adding its file under kartas/ plus one entry here. The
// completeness test asserts this list matches the set of constructors defined in
// the kartas package, so a forgotten entry fails CI.
var definitions = []func() *v1alpha1.Karta{
	kartas.BatchJob,
	kartas.CronJob,
	kartas.Deployment,
	kartas.StatefulSet,
	kartas.Pod,
}

// Catalog is an immutable set of built-in Kartas.
type Catalog struct {
	byGVK map[schema.GroupVersionKind]*v1alpha1.Karta
	all   []*v1alpha1.Karta // sorted by GVK string for deterministic List
}

// New builds the catalog from the definitions list. It panics in case of catalog violation.
func New() *Catalog {
	return newCatalog(definitions)
}

// newCatalog builds a catalog from an explicit definitions list. It exists so
// tests can exercise the duplicate-GVK panic without mutating the package list.
func newCatalog(defs []func() *v1alpha1.Karta) *Catalog {
	c := &Catalog{
		byGVK: make(map[schema.GroupVersionKind]*v1alpha1.Karta, len(defs)),
		all:   make([]*v1alpha1.Karta, 0, len(defs)),
	}
	for _, def := range defs {
		karta := def()
		root := karta.Spec.StructureDefinition.RootComponent
		// Group may be empty (core workloads such as Pod), but Version and Kind are required.
		if root.Kind == nil || root.Kind.Version == "" || root.Kind.Kind == "" {
			panic(fmt.Sprintf("catalog: root component GVK incomplete for Karta %q", karta.Name))
		}
		gvk := RootKey(karta)
		if existing, ok := c.byGVK[gvk]; ok {
			panic(fmt.Sprintf("catalog: duplicate GVK %s defined by %q and %q", gvk, existing.Name, karta.Name))
		}
		c.byGVK[gvk] = karta
		c.all = append(c.all, karta)
	}
	sort.Slice(c.all, func(i, j int) bool {
		return RootKey(c.all[i]).String() < RootKey(c.all[j]).String()
	})
	return c
}

// RootKey returns the workload GVK of a Karta's root component.
func RootKey(k *v1alpha1.Karta) schema.GroupVersionKind {
	kind := k.Spec.StructureDefinition.RootComponent.Kind
	if kind == nil {
		return schema.GroupVersionKind{}
	}
	return schema.GroupVersionKind{Group: kind.Group, Version: kind.Version, Kind: kind.Kind}
}

// Get returns the built-in Karta whose root component matches gvk. The returned
// Karta is a deep copy, so callers may mutate it freely without affecting the
// immutable catalog.
func (c *Catalog) Get(gvk schema.GroupVersionKind) (*v1alpha1.Karta, error) {
	if k, ok := c.byGVK[gvk]; ok {
		return k.DeepCopy(), nil
	}
	return nil, fmt.Errorf("%w: %s", ErrNotFound, gvk)
}

// List returns all built-in Kartas sorted by GVK. Each entry is a deep copy, so
// callers may mutate the slice or the Kartas without affecting the immutable
// catalog.
func (c *Catalog) List() []*v1alpha1.Karta {
	out := make([]*v1alpha1.Karta, len(c.all))
	for i, k := range c.all {
		out[i] = k.DeepCopy()
	}
	return out
}

// defaultCatalog is the package-global instance callers use directly.
var defaultCatalog = New()

// Get resolves a built-in Karta for gvk from the default catalog.
func Get(gvk schema.GroupVersionKind) (*v1alpha1.Karta, error) { return defaultCatalog.Get(gvk) }

// List returns all built-in Kartas from the default catalog, sorted by GVK.
func List() []*v1alpha1.Karta { return defaultCatalog.List() }

// Slug returns the GVK-derived filename stem for a Karta, of the form
// {group-slug}-{kind-lowercase}-{version}. The core (empty) group maps to
// "core" so Pod becomes "core-pod-v1".
func Slug(k *v1alpha1.Karta) (string, error) {
	gvk := RootKey(k)
	if gvk.Version == "" || gvk.Kind == "" {
		return "", fmt.Errorf("root component GVK incomplete for Karta %q", k.Name)
	}
	group := gvk.Group
	if group == "" {
		group = "core"
	}
	return strings.ReplaceAll(group, ".", "-") + "-" + strings.ToLower(gvk.Kind) + "-" + gvk.Version, nil
}

// MarshalYAML renders a Karta into YAML with required header.
func MarshalYAML(k *v1alpha1.Karta) ([]byte, error) {
	body, err := yaml.Marshal(k)
	if err != nil {
		return nil, fmt.Errorf("marshal Karta %q: %w", k.Name, err)
	}
	return append([]byte(yamlHeader), body...), nil
}
