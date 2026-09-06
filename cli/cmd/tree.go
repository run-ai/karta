// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	"github.com/run-ai/karta/cli/pkg/definitions"
	"github.com/run-ai/karta/cli/pkg/generator"
	"github.com/run-ai/karta/cli/pkg/workload"
	"github.com/run-ai/karta/pkg/catalog"
	"github.com/run-ai/karta/pkg/physical"
)

const (
	flagPhysical      = "physical"
	flagTopologyLabel = "topology-label"
)

// treeOptions holds the tree-specific flags, alongside the shared TYPE/NAME
// parsing getOptions already provides.
type treeOptions struct {
	getOptions
	physical       bool
	topologyLabels []string
}

// newTreeCommand builds the "karta tree" command: one workload rendered as a
// hierarchical tree, with live pods attributed to the component and instance
// they actually belong to.
func newTreeCommand() *cobra.Command {
	opts := &treeOptions{}

	cmd := &cobra.Command{
		Use:   "tree TYPE/NAME",
		Short: "Render a workload as a hierarchical tree",
		Long: "Render one workload as a component tree read through its Karta definition, " +
			"with live pods attributed to the component and instance they belong to via the " +
			"definition's own PodSelector.",
		Example: "  karta tree pytorchjob/my-job\n" +
			"  karta tree jobset my-job -n team-nlp --physical",
		Args: func(_ *cobra.Command, args []string) error {
			if err := parseArgs(&opts.getOptions, args); err != nil {
				return exitError{code: ExitUsage, err: err}
			}
			if opts.name == "" {
				return exitError{code: ExitUsage, err: fmt.Errorf("NAME is required")}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTree(cmd, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.physical, flagPhysical, false,
		"resolve the physical layer: node health, topology domain, and per-device identity where DRA is in use")
	cmd.Flags().StringSliceVar(&opts.topologyLabels, flagTopologyLabel, nil,
		"node labels that name a topology domain, tried in order (default nvidia.com/gpu.clique, topology.kubernetes.io/zone)")

	return cmd
}

func runTree(cmd *cobra.Command, opts *treeOptions) error {
	ctx := cmd.Context()
	access := clusterAccess()

	namespace, _, err := ResolvedNamespace(access)
	if err != nil {
		return fmt.Errorf("resolve namespace: %w", err)
	}

	resolver, warnings := loadDefinitions(ctx, access)
	for _, warning := range warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning.Message)
	}
	if len(resolver.List()) == 0 {
		return exitError{code: ExitNotFound, err: fmt.Errorf(
			"no Karta definitions available (catalog empty and no cluster definitions)")}
	}

	mapper, err := access.ToRESTMapper()
	if err != nil {
		return fmt.Errorf("kubernetes discovery: %w", err)
	}
	dyn, err := newDynamicClient(access)
	if err != nil {
		return err
	}

	target, err := resolveTarget(&opts.getOptions, resolver, mapper)
	if err != nil {
		return err
	}

	obj, err := getOne(ctx, dyn, mapper, target, namespace, opts.name)
	if err != nil {
		return err
	}

	pods, err := listPods(ctx, dyn, namespace)
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}
	attributor := workload.NewPodAttributor(dyn, mapper)
	attributed := attributor.Filter(ctx, pods, obj.GetUID())

	view, err := workload.ResolveTree(ctx, obj, target, attributed)
	if err != nil {
		return fmt.Errorf("resolve tree: %w", err)
	}

	if opts.physical {
		snap := physical.Resolve(ctx, dyn, mapper, attributed, physical.Options{TopologyLabels: opts.topologyLabels})
		for _, warning := range snap.Warnings {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
		}
		workload.EnrichTree(view, snap)
	}

	return generator.Tree(cmd.OutOrStdout(), view, resolveStyle(cmd))
}

// resolveStyle turns --color into a generator.Style: auto detects a TTY,
// always/never force it either way regardless of an invalid value, which
// falls back to auto.
func resolveStyle(cmd *cobra.Command) generator.Style {
	switch colorFlag(cmd) {
	case "always":
		return generator.ForceStyle()
	case "never":
		return generator.PlainStyle()
	default:
		return generator.AutoStyle(cmd.OutOrStdout())
	}
}

// getOne fetches the single named object for target, matching the error
// shape collect() uses for a named get in "karta get".
func getOne(
	ctx context.Context, dyn dynamic.Interface, mapper meta.RESTMapper, target definitions.Definition, namespace, name string,
) (*unstructured.Unstructured, error) {
	gvk := catalog.RootKey(target.Karta)
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", gvk.Kind, err)
	}
	obj, err := dyn.Resource(mapping.Resource).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	switch {
	case err == nil:
		return obj, nil
	case apierrors.IsNotFound(err):
		return nil, exitError{code: ExitError, err: fmt.Errorf("%s %q not found%s", gvk.Kind, name, inNamespace(namespace))}
	default:
		return nil, exitError{code: ExitError, err: fmt.Errorf("get %s %q: %w", gvk.Kind, name, err)}
	}
}

// listPods lists every pod in namespace once, decoded to the typed shape
// PodAttributor and ResolveTree both need.
func listPods(ctx context.Context, dyn dynamic.Interface, namespace string) ([]corev1.Pod, error) {
	list, err := dyn.Resource(podsGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods := make([]corev1.Pod, 0, len(list.Items))
	for i := range list.Items {
		var pod corev1.Pod
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[i].Object, &pod); err != nil {
			return nil, fmt.Errorf("decode pod %s: %w", list.Items[i].GetName(), err)
		}
		pods = append(pods, pod)
	}
	return pods, nil
}
