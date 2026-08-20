// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package definitions answers "which Karta definition describes this workload?"
// for the CLI. It merges the built-in community catalog with the Karta custom
// resources installed in a cluster, so a command can resolve a definition by
// workload GVK, by kind, or by name without knowing where the definition came
// from. Cluster definitions take precedence over community ones for the same
// root GVK.
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

// Origin records where a definition was read from.
type Origin string

const (
	// OriginCommunity marks a definition shipped in the built-in catalog.
	OriginCommunity Origin = "community"

	// OriginCluster marks a definition read from a cluster as a Karta custom resource.
	OriginCluster Origin = "cluster"
)

// Definition is a Karta together with the source it was read from.
type Definition struct {
	Karta  *v1alpha1.Karta
	Origin Origin
}

// Collision records a root GVK claimed by more than one definition from the same source.
type Collision struct {
	GVK      schema.GroupVersionKind
	Winner   string   // metadata.name of the definition that won
	Shadowed []string // metadata.names that were overridden, name-sorted
}

// ErrNotFound is returned by Resolve when no definition claims a GVK.
var ErrNotFound = errors.New("definitions: no Karta definition for GVK")

// ErrNameNotFound is returned by ByName when no definition carries a name.
var ErrNameNotFound = errors.New("definitions: no Karta definition named")

// Resolver is an immutable lookup over the merged community and cluster definitions.
type Resolver struct {
	// effective holds the single definition that wins for a GVK, which is what
	// Resolve answers with.
	effective map[schema.GroupVersionKind]Definition

	// listing holds every definition that is visible for a GVK. It differs from
	// effective because two definitions from the same source may both claim a GVK
	// and both deserve to be listed, while Resolve must still pick exactly one.
	listing map[schema.GroupVersionKind][]Definition

	collisions []Collision
}

// New merges community and cluster definitions into a Resolver. Community is
// indexed first so a cluster definition overrides a community one claiming the
// same root GVK. Nothing here validates definitions.
func New(community, cluster []*v1alpha1.Karta) *Resolver {
	r := &Resolver{
		effective: make(map[schema.GroupVersionKind]Definition, len(community)+len(cluster)),
		listing:   make(map[schema.GroupVersionKind][]Definition, len(community)+len(cluster)),
	}
	r.index(community, OriginCommunity)
	r.index(cluster, OriginCluster)
	slices.SortFunc(r.collisions, func(a, b Collision) int {
		return strings.Compare(a.GVK.String(), b.GVK.String())
	})
	return r
}

// index adds one source to the resolver. Sorting by name makes the winner
// deterministic: the last write wins, so the last name alphabetically is the
// definition Resolve returns.
func (r *Resolver) index(kartas []*v1alpha1.Karta, origin Origin) {
	sorted := slices.Clone(kartas)
	slices.SortFunc(sorted, func(a, b *v1alpha1.Karta) int {
		return strings.Compare(a.Name, b.Name)
	})

	// Both maps are scoped to this call, which is what makes collisions and the
	// cross-source replacement same-source concerns only.
	claimants := make(map[schema.GroupVersionKind][]string, len(sorted))
	replaced := make(map[schema.GroupVersionKind]bool, len(sorted))

	for _, karta := range sorted {
		root := karta.Spec.StructureDefinition.RootComponent.Kind
		// A zero GVK cannot serve as a map key, so an incomplete root is dropped
		// rather than indexed. Group may be empty for core workloads such as Pod.
		if root == nil || root.Version == "" || root.Kind == "" {
			continue
		}
		gvk := catalog.RootKey(karta)
		def := Definition{Karta: karta, Origin: origin}

		r.effective[gvk] = def
		if !replaced[gvk] {
			// The first claimant from this source supersedes whatever an earlier
			// source listed for the GVK; later same-source claimants stack on top.
			replaced[gvk] = true
			r.listing[gvk] = nil
		}
		r.listing[gvk] = append(r.listing[gvk], def)
		claimants[gvk] = append(claimants[gvk], karta.Name)
	}

	for gvk, names := range claimants {
		if len(names) < 2 {
			continue
		}
		r.collisions = append(r.collisions, Collision{
			GVK:      gvk,
			Winner:   names[len(names)-1],
			Shadowed: slices.Clone(names[:len(names)-1]),
		})
	}
}

// Resolve returns the definition that wins for gvk, or ErrNotFound.
func (r *Resolver) Resolve(gvk schema.GroupVersionKind) (Definition, error) {
	def, ok := r.effective[gvk]
	if !ok {
		return Definition{}, fmt.Errorf("%w: %s", ErrNotFound, gvk)
	}
	return def.deepCopy(), nil
}

// List returns every visible definition sorted by GVK then name. A GVK claimed
// by several same-source definitions contributes one entry per claimant.
func (r *Resolver) List() []Definition {
	all := r.ordered()
	out := make([]Definition, 0, len(all))
	for _, def := range all {
		out = append(out, def.deepCopy())
	}
	return out
}

// ByKind returns every definition whose root kind matches kind, kubectl-style:
// case-insensitive, tolerating one trailing "s" on either side. A kind can be
// covered at several API versions, so the result is a slice; no match yields an
// empty slice rather than an error.
func (r *Resolver) ByKind(kind string) []Definition {
	query := strings.ToLower(kind)
	singular := strings.TrimSuffix(query, "s")
	plural := query + "s"

	out := make([]Definition, 0)
	for _, def := range r.ordered() {
		root := strings.ToLower(catalog.RootKey(def.Karta).Kind)
		if root == query || root == singular || root == plural {
			out = append(out, def.deepCopy())
		}
	}
	return out
}

// ByName returns the definition called name. The match is exact and
// case-sensitive because names are identifiers copied from list output.
func (r *Resolver) ByName(name string) (Definition, error) {
	for _, def := range r.ordered() {
		if def.Karta.Name == name {
			return def.deepCopy(), nil
		}
	}
	return Definition{}, fmt.Errorf("%w: %q", ErrNameNotFound, name)
}

// Collisions returns the root GVKs claimed by more than one definition from the
// same source, sorted by GVK. A cluster definition overriding a community one is
// the documented precedence rule, not a collision.
func (r *Resolver) Collisions() []Collision {
	out := make([]Collision, 0, len(r.collisions))
	for _, c := range r.collisions {
		out = append(out, Collision{GVK: c.GVK, Winner: c.Winner, Shadowed: slices.Clone(c.Shadowed)})
	}
	return out
}

// ordered returns the listed definitions sorted by GVK then name, still sharing
// the resolver's Kartas. Entries within one GVK are already name-sorted because
// index appends them in that order, and a source replaces rather than interleaves.
func (r *Resolver) ordered() []Definition {
	gvks := slices.SortedFunc(maps.Keys(r.listing), func(a, b schema.GroupVersionKind) int {
		return strings.Compare(a.String(), b.String())
	})
	out := make([]Definition, 0, len(r.listing))
	for _, gvk := range gvks {
		out = append(out, r.listing[gvk]...)
	}
	return out
}

// deepCopy detaches a Definition from the resolver, matching the contract
// pkg/catalog establishes: a caller may mutate what it was handed.
func (d Definition) deepCopy() Definition {
	return Definition{Karta: d.Karta.DeepCopy(), Origin: d.Origin}
}
