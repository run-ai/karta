// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package definitions

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog"
)

type Origin string

const (
	OriginCommunity Origin = "community"
	OriginCluster   Origin = "cluster"
)

var ErrNotFound = errors.New("definitions: no Karta definition for GVK")

var ErrNameNotFound = errors.New("definitions: no Karta definition named")

var ErrAmbiguous = errors.New("definitions: more than one Karta definition for GVK")

// mappable reports the root GVK a definition can be looked up by.
func mappable(d Definition) (schema.GroupVersionKind, bool) {
	gvk := catalog.RootKey(d.Karta)
	return gvk, gvk.Version != "" && gvk.Kind != ""
}

// Definition is a Karta together with the source it was read from.
type Definition struct {
	Karta  *v1alpha1.Karta
	Origin Origin
}

// Collision records a root GVK claimed by more than one definition from the same
// source.
type Collision struct {
	GVK   schema.GroupVersionKind
	Names []string // metadata.names claiming this GVK, name-sorted
}

// Resolver is an immutable lookup over the merged community and cluster definitions.
type Resolver struct {
	ordered []Definition
}

// New merges community and cluster definitions into a Resolver. Community is
// indexed first so a cluster definition overrides a community one claiming the
// same root GVK.
func New(community, cluster []*v1alpha1.Karta) *Resolver {
	r := &Resolver{}
	// Keyed only while merging: a later source replaces an earlier one wholesale
	// for a GVK, which is what makes cluster take precedence over the catalog.
	listing := make(map[schema.GroupVersionKind][]Definition, len(community)+len(cluster))
	unmapped := index(listing, community, OriginCommunity)
	unmapped = append(unmapped, index(listing, cluster, OriginCluster)...)

	gvks := slices.SortedFunc(maps.Keys(listing), func(a, b schema.GroupVersionKind) int {
		return strings.Compare(a.String(), b.String())
	})
	r.ordered = make([]Definition, 0, len(listing)+len(unmapped))
	for _, gvk := range gvks {
		r.ordered = append(r.ordered, listing[gvk]...)
	}
	slices.SortFunc(unmapped, func(a, b Definition) int {
		return strings.Compare(a.Karta.Name, b.Karta.Name)
	})
	r.ordered = append(r.ordered, unmapped...)
	return r
}

// index adds one source to the resolver. Sorting by name keeps the outcome
// deterministic.
func index(listing map[schema.GroupVersionKind][]Definition, kartas []*v1alpha1.Karta, origin Origin) []Definition {
	sorted := slices.Clone(kartas)
	slices.SortFunc(sorted, func(a, b *v1alpha1.Karta) int {
		return strings.Compare(a.Name, b.Name)
	})

	var unmapped []Definition
	claimed := make(map[schema.GroupVersionKind][]Definition, len(sorted))
	for _, karta := range sorted {
		def := Definition{Karta: karta, Origin: origin}
		gvk, ok := mappable(def)
		if !ok {
			unmapped = append(unmapped, def)
			continue
		}
		claimed[gvk] = append(claimed[gvk], def)
	}

	for gvk, defs := range claimed {
		// Assignment, not append: this source replaces an earlier one for the GVK.
		listing[gvk] = defs
	}
	return unmapped
}

// Resolve returns the one definition covering gvk.
func (r *Resolver) Resolve(gvk schema.GroupVersionKind) (Definition, error) {
	var defs []Definition
	for _, def := range r.ordered {
		if root, ok := mappable(def); ok && root == gvk {
			defs = append(defs, def)
		}
	}
	switch len(defs) {
	case 0:
		return Definition{}, fmt.Errorf("%w: %s", ErrNotFound, gvk)
	case 1:
		return defs[0], nil
	default:
		names := make([]string, 0, len(defs))
		for _, def := range defs {
			names = append(names, fmt.Sprintf("%q", def.Karta.Name))
		}
		return Definition{}, fmt.Errorf("%w: %s: %s", ErrAmbiguous, gvk, strings.Join(names, ", "))
	}
}

// List returns every visible definition sorted by GVK then name, one entry per
// claimant, with definitions that name no GVK last.
func (r *Resolver) List() []Definition {
	return r.ordered
}

// ByRootKind matches the Kind segment of a GVK, not a lookup of its own: callers
// narrow by version and group themselves.
func (r *Resolver) ByRootKind(kind string) []Definition {
	query := strings.ToLower(kind)
	singular := strings.TrimSuffix(query, "s")
	plural := query + "s"

	out := make([]Definition, 0)
	for _, def := range r.ordered {
		gvk, ok := mappable(def)
		if !ok {
			continue
		}
		root := strings.ToLower(gvk.Kind)
		if root == query || root == singular || root == plural {
			out = append(out, def)
		}
	}
	return out
}

// ByName returns the definition called name, matched exactly: names are
// identifiers copied from list output.
func (r *Resolver) ByName(name string) (Definition, error) {
	for _, def := range r.ordered {
		if def.Karta.Name == name {
			return def, nil
		}
	}
	return Definition{}, fmt.Errorf("%w: %q", ErrNameNotFound, name)
}

// Collisions returns root GVKs claimed by more than one definition from the same
// source.
func (r *Resolver) Collisions() []Collision {
	var out []Collision
	var run schema.GroupVersionKind
	var names []string
	flush := func() {
		if len(names) > 1 {
			out = append(out, Collision{GVK: run, Names: names})
		}
		names = nil
	}
	for _, def := range r.ordered {
		gvk, ok := mappable(def)
		if !ok || gvk != run {
			flush()
			run = gvk
		}
		if ok {
			names = append(names, def.Karta.Name)
		}
	}
	flush()
	return out
}
