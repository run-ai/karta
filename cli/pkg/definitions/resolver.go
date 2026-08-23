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

var ErrNotFound = errors.New("definitions: no Karta definition for GVK")

var ErrNameNotFound = errors.New("definitions: no Karta definition named")

// Resolver is an immutable lookup over the merged community and cluster definitions.
type Resolver struct {
	effective map[schema.GroupVersionKind]Definition

	listing map[schema.GroupVersionKind][]Definition

	ordered []Definition

	collisions []Collision
}

// New merges community and cluster definitions into a Resolver. Community is
// indexed first so a cluster definition overrides a community one claiming the
// same root GVK.
func New(community, cluster []*v1alpha1.Karta) *Resolver {
	r := &Resolver{
		effective: make(map[schema.GroupVersionKind]Definition, len(community)+len(cluster)),
		listing:   make(map[schema.GroupVersionKind][]Definition, len(community)+len(cluster)),
	}
	unmapped := r.index(community, OriginCommunity)
	unmapped = append(unmapped, r.index(cluster, OriginCluster)...)
	slices.SortFunc(r.collisions, func(a, b Collision) int {
		return strings.Compare(a.GVK.String(), b.GVK.String())
	})

	gvks := slices.SortedFunc(maps.Keys(r.listing), func(a, b schema.GroupVersionKind) int {
		return strings.Compare(a.String(), b.String())
	})
	r.ordered = make([]Definition, 0, len(r.listing)+len(unmapped))
	for _, gvk := range gvks {
		r.ordered = append(r.ordered, r.listing[gvk]...)
	}
	slices.SortFunc(unmapped, func(a, b Definition) int {
		return strings.Compare(a.Karta.Name, b.Karta.Name)
	})
	r.ordered = append(r.ordered, unmapped...)
	return r
}

// index adds one source to the resolver. Sorting by name keeps the outcome
// deterministic.
func (r *Resolver) index(kartas []*v1alpha1.Karta, origin Origin) []Definition {
	sorted := slices.Clone(kartas)
	slices.SortFunc(sorted, func(a, b *v1alpha1.Karta) int {
		return strings.Compare(a.Name, b.Name)
	})

	var unmapped []Definition
	claimed := make(map[schema.GroupVersionKind][]Definition, len(sorted))
	for _, karta := range sorted {
		root := karta.Spec.StructureDefinition.RootComponent.Kind
		// A zero GVK cannot serve as a map key, so an incomplete root is set aside
		// rather than indexed. Group may be empty for core workloads such as Pod.
		if root == nil || root.Version == "" || root.Kind == "" {
			unmapped = append(unmapped, Definition{Karta: karta, Origin: origin})
			continue
		}
		gvk := catalog.RootKey(karta)
		def := Definition{Karta: karta, Origin: origin}

		r.effective[gvk] = def
		claimed[gvk] = append(claimed[gvk], def)
	}

	for gvk, defs := range claimed {
		// Assignment, not append: this source replaces an earlier one for the GVK.
		r.listing[gvk] = defs
		if len(defs) < 2 {
			continue
		}
		names := make([]string, 0, len(defs))
		for _, def := range defs {
			names = append(names, def.Karta.Name)
		}
		r.collisions = append(r.collisions, Collision{GVK: gvk, Names: names})
	}
	return unmapped
}

// Resolve returns the definition that wins for gvk, or ErrNotFound.
func (r *Resolver) Resolve(gvk schema.GroupVersionKind) (Definition, error) {
	def, ok := r.effective[gvk]
	if !ok {
		return Definition{}, fmt.Errorf("%w: %s", ErrNotFound, gvk)
	}
	return def, nil
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
		root := strings.ToLower(catalog.RootKey(def.Karta).Kind)
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
	return r.collisions
}
