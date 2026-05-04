// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package definitions ships the community Karta definitions that the karta
// CLI knows about out of the box. Each definition is embedded at build time
// and indexed by the workload GroupVersionKind it targets, so commands like
// `karta workload tree` can resolve a definition from the workload object's
// GVK without touching the cluster.
package definitions

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

//go:embed community/*.yaml
var communityFS embed.FS

// Registry holds the parsed community definitions, keyed by the GVK they target.
type Registry struct {
	byGVK map[schema.GroupVersionKind]*v1alpha1.Karta
}

// Lookup returns the community definition for gvk, or nil when none is known.
func (r *Registry) Lookup(gvk schema.GroupVersionKind) *v1alpha1.Karta {
	if r == nil {
		return nil
	}
	return r.byGVK[gvk]
}

// All returns every loaded community definition.
func (r *Registry) All() []*v1alpha1.Karta {
	out := make([]*v1alpha1.Karta, 0, len(r.byGVK))
	for _, k := range r.byGVK {
		out = append(out, k)
	}
	return out
}

var (
	once     sync.Once
	loaded   *Registry
	loadErr  error
)

// Load parses every embedded community definition and indexes it by GVK.
// The result is cached for the lifetime of the process.
func Load() (*Registry, error) {
	once.Do(func() {
		reg := &Registry{byGVK: map[schema.GroupVersionKind]*v1alpha1.Karta{}}
		err := fs.WalkDir(communityFS, "community", func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
				return nil
			}
			data, err := communityFS.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			k := &v1alpha1.Karta{}
			if err := yaml.Unmarshal(data, k); err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			gvk := rootGVK(k)
			if gvk.Kind == "" {
				return fmt.Errorf("%s: root component has no kind", path)
			}
			reg.byGVK[gvk] = k
			return nil
		})
		if err != nil {
			loadErr = err
			return
		}
		loaded = reg
	})
	return loaded, loadErr
}

func rootGVK(k *v1alpha1.Karta) schema.GroupVersionKind {
	root := k.Spec.StructureDefinition.RootComponent
	if root.Kind == nil {
		return schema.GroupVersionKind{}
	}
	return schema.GroupVersionKind{
		Group:   root.Kind.Group,
		Version: root.Kind.Version,
		Kind:    root.Kind.Kind,
	}
}
