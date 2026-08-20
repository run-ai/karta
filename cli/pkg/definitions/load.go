// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package definitions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/run-ai/karta/pkg/catalog"
)

// Load builds the resolver every CLI command looks definitions up in. It seeds
// the built-in catalog and then best-effort merges the cluster's Karta
// definitions on top. A cluster that cannot be reached degrades to the built-in
// catalog instead of failing the command, so Load returns no error. Warnings go
// to warnOut, which callers point at stderr so stdout stays parseable.
func Load(ctx context.Context, rcg genericclioptions.RESTClientGetter, warnOut io.Writer) *Resolver {
	cluster, err := FetchCluster(ctx, rcg)
	if err != nil {
		classify(err, warnOut)
		cluster = nil
	}

	resolver := New(catalog.List(), cluster)
	for _, c := range resolver.Collisions() {
		// Only cluster definitions can collide here: the built-in catalog rejects
		// duplicate root GVKs when it is constructed.
		shadowed := make([]string, 0, len(c.Shadowed))
		for _, name := range c.Shadowed {
			shadowed = append(shadowed, fmt.Sprintf("%q", name))
		}
		fmt.Fprintf(warnOut, "warning: %d cluster Karta definitions claim %s; using %q and ignoring %s\n",
			len(c.Shadowed)+1, c.GVK, c.Winner, strings.Join(shadowed, ", "))
	}
	return resolver
}

// classify turns a cluster read failure into either silence or a warning. It is
// separate from Load because Load builds its own client and cannot be handed a
// fake, so this is where the degradation behaviour is exercised.
func classify(err error, warnOut io.Writer) {
	// clientcmd.IsEmptyConfig type-switches on the concrete error and never
	// unwraps, so it reports false for a cause wrapped with %w. Walk the chain by
	// hand, otherwise running with no kubeconfig would warn instead of quietly
	// falling back to the built-in catalog.
	for cause := err; cause != nil; cause = errors.Unwrap(cause) {
		if clientcmd.IsEmptyConfig(cause) {
			return
		}
	}

	switch {
	case apierrors.IsNotFound(err):
		// The Karta CRD is not installed, which is the expected out-of-the-box
		// state and not worth telling the user about.
	case apierrors.IsForbidden(err):
		fmt.Fprintln(warnOut, "warning: not allowed to list kartas.run.ai; showing built-in definitions only. "+
			"Ask a cluster administrator for \"list\" permission on kartas.run.ai to include cluster definitions.")
	default:
		fmt.Fprintf(warnOut, "warning: could not read Karta definitions from the cluster: %v; showing built-in definitions only\n", err)
	}
}
