// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/run-ai/karta/cli/pkg/definitions"
	"github.com/run-ai/karta/cli/pkg/generator"
	"github.com/run-ai/karta/cli/pkg/workload"
	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog"
)

const (
	flagPhase     = "phase"
	flagSelector  = "selector"
	flagChunkSize = "chunk-size"
)

// defaultChunkSize follows the kubectl convention for list page size.
const defaultChunkSize = 500

// getOptions holds the resolved inputs for one run of the get command.
type getOptions struct {
	typeToken string
	name      string
	phases    []string
	selector  string
	chunkSize int64
}

// newGetCommand builds the "karta get" command: one row per workload root.
func newGetCommand() *cobra.Command {
	opts := &getOptions{}

	cmd := &cobra.Command{
		Use:   "get TYPE[/NAME] [NAME]",
		Short: "List workloads of a type",
		Long: "List workloads with a normalized phase, their semantic component breakdown, and " +
			"aggregate requested GPUs, read through the Karta definition that covers each type.\n\n" +
			"Type matching is lenient: case-insensitive, singular or plural, and kubectl short " +
			"names all resolve.\n\n" +
			"Counts and GPUs come from the workload spec, so no pods are listed. -o wide adds the " +
			"ORIGIN of the resolving definition; a NODES column arrives with the describe command.",
		Example: "  # All JobSets in the current namespace\n" +
			"  karta get jobset\n\n" +
			"  # Failed JobSets\n" +
			"  karta get jobset --phase Failed\n\n" +
			"  # Degraded or failed JobSets matching a selector, as JSON\n" +
			"  karta get jobset --phase Degraded --phase Failed -l team=nlp -o json",
		Args: func(_ *cobra.Command, args []string) error {
			if err := parseArgs(opts, args); err != nil {
				return exitError{code: ExitUsage, err: err}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGet(cmd, opts)
		},
	}

	cmd.Flags().StringSliceVar(&opts.phases, flagPhase, nil,
		"Filter by normalized phase; repeatable. Applied after resolution, so it does not reduce API cost")
	cmd.Flags().StringVarP(&opts.selector, flagSelector, "l", "",
		"Label selector on the workload root, kubectl syntax")
	cmd.Flags().Int64Var(&opts.chunkSize, flagChunkSize, defaultChunkSize,
		"API list page size; 0 lists without paging. Bounds memory and API pressure, not time to first row")

	return cmd
}

// parseArgs accepts the three kubectl argument forms: TYPE, TYPE/NAME, and
// TYPE NAME.
func parseArgs(opts *getOptions, args []string) error {
	switch {
	case len(args) > 2:
		return fmt.Errorf("accepts at most 2 arguments, received %d", len(args))
	case len(args) == 0:
		return fmt.Errorf("a TYPE argument is required")
	}

	var qualified bool
	opts.typeToken, opts.name, qualified = strings.Cut(args[0], "/")
	if qualified && opts.name == "" {
		return fmt.Errorf("NAME is empty in %q", args[0])
	}
	if len(args) == 2 {
		if opts.name != "" {
			return fmt.Errorf("NAME given twice: as %q and as %q", args[0], args[1])
		}
		if args[1] == "" {
			return fmt.Errorf("NAME is empty")
		}
		opts.name = args[1]
	}
	if strings.Contains(opts.name, "/") {
		return fmt.Errorf("NAME must not contain %q: %q", "/", opts.name)
	}
	if opts.typeToken == "" {
		return fmt.Errorf("TYPE is empty in %q", args[0])
	}
	return nil
}

func runGet(cmd *cobra.Command, opts *getOptions) error {
	ctx := cmd.Context()

	if opts.chunkSize < 0 {
		return exitError{code: ExitUsage, err: fmt.Errorf("--%s must not be negative", flagChunkSize)}
	}
	if _, err := labels.Parse(opts.selector); err != nil {
		return exitError{code: ExitUsage, err: fmt.Errorf("--%s: %w", flagSelector, err)}
	}
	if err := validatePhases(opts.phases); err != nil {
		return exitError{code: ExitUsage, err: err}
	}
	if opts.selector != "" && opts.name != "" {
		return exitError{code: ExitUsage, err: fmt.Errorf(
			"--%s cannot be combined with a NAME", flagSelector)}
	}

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

	target, err := resolveTarget(opts, resolver, mapper)
	if err != nil {
		return err
	}

	views, searched, listWarnings, err := collect(ctx, dyn, mapper, target, namespace, opts)
	for _, warning := range listWarnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
	}
	if err != nil {
		return err
	}

	if len(views) > 0 {
		if podWarning := attributePods(ctx, dyn, mapper, searched, views); podWarning != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", podWarning)
		}
	}

	// Newest first, so a freshly submitted workload is at the top.
	sort.SliceStable(views, func(i, j int) bool {
		if !views[i].CreatedAt.Equal(views[j].CreatedAt) {
			return views[i].CreatedAt.After(views[j].CreatedAt)
		}
		return views[i].Name < views[j].Name
	})

	return generator.Render(cmd.OutOrStdout(), cmd.ErrOrStderr(), views, generator.Options{
		Output:    outputFormat(cmd),
		Namespace: searched,
		// An empty namespace means the type is cluster-scoped, so the search
		// spanned the cluster.
		AllNamespaces: searched == "",
	})
}

// validatePhases checks values against the ResourceStatus enum, which Entries()
// reports without Undefined.
func validatePhases(phases []string) error {
	allowed := []string{string(v1alpha1.UndefinedStatus)}
	for _, entry := range (v1alpha1.StatusMappings{}).Entries() {
		allowed = append(allowed, string(entry.Status))
	}

	for _, phase := range phases {
		if !slices.ContainsFunc(allowed, func(a string) bool { return strings.EqualFold(a, phase) }) {
			return fmt.Errorf("--%s %q: must be one of %s", flagPhase, phase, strings.Join(allowed, ", "))
		}
	}
	return nil
}

// resolveTarget maps the requested type to the definition that covers it.
func resolveTarget(
	opts *getOptions,
	resolver *definitions.Resolver,
	mapper meta.RESTMapper,
) (definitions.Definition, error) {
	// Discovery resolves short names, plurals and qualified forms. A dotted
	// token has two readings, and ParseResourceArg leaves the choice to callers.
	qualified, groupResource := schema.ParseResourceArg(strings.ToLower(opts.typeToken))
	candidates := []schema.GroupVersionResource{groupResource.WithVersion("")}
	if qualified != nil {
		candidates = append([]schema.GroupVersionResource{*qualified}, candidates...)
	}
	for _, candidate := range candidates {
		def, ok, err := resolveVia(mapper, resolver, candidate)
		if err != nil {
			return definitions.Definition{}, err
		}
		if ok {
			return def, nil
		}
	}

	// Without discovery only an exact Kind matches, but a type whose CRD is
	// absent must still resolve, including the qualified form suggested below.
	switch matches := matchDefinitions(resolver, opts.typeToken); len(matches) {
	case 0:
		return definitions.Definition{}, exitError{code: ExitNotFound,
			err: fmt.Errorf("no Karta definition covers %q", opts.typeToken)}
	case 1:
		return matches[0], nil
	default:
		return definitions.Definition{}, exitError{code: ExitUsage,
			err: ambiguous(opts.typeToken, matches)}
	}
}

// ambiguous explains a type token several definitions claim. Identical root
// GVKs cannot be retried with a qualified token, so those name the definitions.
func ambiguous(token string, matches []definitions.Definition) error {
	tokens := make([]string, 0, len(matches))
	for _, match := range matches {
		tokens = append(tokens, qualifiedToken(catalog.RootKey(match.Karta)))
	}
	slices.Sort(tokens)
	if distinct := slices.Compact(slices.Clone(tokens)); len(distinct) > 1 {
		return fmt.Errorf("%q matches more than one Karta definition, retry with one of: %s",
			token, strings.Join(distinct, ", "))
	}

	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, fmt.Sprintf("%q", match.Karta.Name))
	}
	slices.Sort(names)
	return fmt.Errorf("%d Karta definitions claim %s: %s; remove one from the cluster",
		len(names), tokens[0], strings.Join(names, ", "))
}

// qualifiedToken renders the kind.version.group form that names one definition
// unambiguously.
func qualifiedToken(gvk schema.GroupVersionKind) string {
	return fmt.Sprintf("%s.%s.%s", strings.ToLower(gvk.Kind), gvk.Version, gvk.Group)
}

// matchDefinitions matches a type token against the definitions themselves,
// narrowing by group and version when the token carries them.
func matchDefinitions(resolver *definitions.Resolver, token string) []definitions.Definition {
	qualified, groupKind := schema.ParseKindArg(token)

	matches := resolver.ByRootKind(groupKind.Kind)
	if groupKind.Group == "" {
		return matches
	}

	narrowed := make([]definitions.Definition, 0, len(matches))
	for _, match := range matches {
		switch gvk := catalog.RootKey(match.Karta); {
		case qualified != nil && strings.EqualFold(gvk.Group, qualified.Group) &&
			strings.EqualFold(gvk.Version, qualified.Version):
			narrowed = append(narrowed, match)
		case strings.EqualFold(gvk.Group, groupKind.Group):
			narrowed = append(narrowed, match)
		}
	}
	return narrowed
}

// resolveVia turns a partial resource into a kind and looks it up. A miss lets
// the caller try the next reading; a collision must not fall through.
func resolveVia(
	mapper meta.RESTMapper,
	resolver *definitions.Resolver,
	gvr schema.GroupVersionResource,
) (definitions.Definition, bool, error) {
	gvk, err := mapper.KindFor(gvr)
	if err != nil {
		return definitions.Definition{}, false, nil
	}

	def, err := resolver.Resolve(gvk)
	switch {
	case err == nil:
		return def, true, nil
	case errors.Is(err, definitions.ErrAmbiguous):
		return definitions.Definition{}, false, exitError{code: ExitUsage, err: err}
	default:
		return definitions.Definition{}, false, nil
	}
}

// collect lists the target type and resolves every object into a view. A single
// object that fails to resolve degrades to a warning; the listing itself does not.
func collect(
	ctx context.Context,
	dyn dynamic.Interface,
	mapper meta.RESTMapper,
	def definitions.Definition,
	namespace string,
	opts *getOptions,
) (views []workload.View, searched string, warnings []string, err error) {
	gvk := catalog.RootKey(def.Karta)

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	switch {
	case meta.IsNoMatchError(err) && opts.name != "":
		return nil, namespace, nil, exitError{code: ExitError,
			err: fmt.Errorf("%s is not installed in this cluster", gvk.Kind)}
	case meta.IsNoMatchError(err):
		// No object of this type can exist, so an empty result is the answer.
		return nil, namespace,
			[]string{fmt.Sprintf("%s is not installed in this cluster", gvk.Kind)}, nil
	case err != nil:
		// Discovery failed, so reporting this type as "not installed" would be
		// misleading.
		return nil, namespace, nil, fmt.Errorf("discover %s: %w", gvk.Kind, err)
	}

	// A cluster-scoped root is searched cluster-wide, so the namespace must drop
	// out of the diagnostics too.
	if mapping.Scope.Name() == meta.RESTScopeNameRoot {
		namespace = ""
	}

	objects, err := list(ctx, dyn, mapping, namespace, opts)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err) && opts.name != "":
		return nil, namespace, nil, exitError{code: ExitError,
			err: fmt.Errorf("%s %q not found%s", gvk.Kind, opts.name, inNamespace(namespace))}
	case opts.name != "":
		return nil, namespace, nil, exitError{code: ExitError,
			err: fmt.Errorf("get %s %q: %w", gvk.Kind, opts.name, err)}
	case apierrors.IsForbidden(err):
		// Being denied is not an empty result: the caller cannot be told either
		// way, so it must not read as "no workloads found".
		return nil, namespace, nil, exitError{code: ExitError,
			err: fmt.Errorf("not allowed to list %s: %w", gvk.Kind, err)}
	default:
		return nil, namespace, nil, fmt.Errorf("list %s: %w", gvk.Kind, err)
	}

	for i := range objects {
		object := &objects[i]
		view, err := workload.Resolve(ctx, object, def)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("could not resolve %s/%s: %v",
				gvk.Kind, object.GetName(), err))
			continue
		}
		if !matchesPhase(view, opts.phases) {
			continue
		}
		views = append(views, *view)
	}

	return views, namespace, warnings, nil
}

// list reads one type, following continuation tokens. A named request is a Get,
// so a missing name is NotFound rather than an empty list.
func list(
	ctx context.Context,
	dyn dynamic.Interface,
	mapping *meta.RESTMapping,
	namespace string,
	opts *getOptions,
) ([]unstructured.Unstructured, error) {
	client := dyn.Resource(mapping.Resource).Namespace(namespace)

	if opts.name != "" {
		object, err := client.Get(ctx, opts.name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return []unstructured.Unstructured{*object}, nil
	}

	var objects []unstructured.Unstructured
	options := metav1.ListOptions{Limit: opts.chunkSize, LabelSelector: opts.selector}
	for {
		page, err := client.List(ctx, options)
		if err != nil {
			return nil, err
		}
		objects = append(objects, page.Items...)

		options.Continue = page.GetContinue()
		if options.Continue == "" {
			return objects, nil
		}
	}
}

// inNamespace renders the namespace clause, which a cluster-wide search omits.
func inNamespace(namespace string) string {
	if namespace == "" {
		return ""
	}
	return " in namespace " + namespace
}

func matchesPhase(view *workload.View, requested []string) bool {
	if len(requested) == 0 {
		return true
	}
	for _, want := range requested {
		if slices.ContainsFunc(view.Phases, func(got string) bool { return strings.EqualFold(got, want) }) {
			return true
		}
	}
	return false
}

// loadDefinitions is a variable so tests can supply a definition set.
var loadDefinitions = definitions.Load

// newDynamicClient is a variable so tests can inject a fake cluster.
var newDynamicClient = func(rcg genericclioptions.RESTClientGetter) (dynamic.Interface, error) {
	cfg, err := RESTConfig(rcg)
	if err != nil {
		return nil, err
	}
	// Set per-config: a global handler would land in machine-readable stderr.
	cfg.WarningHandler = rest.NoWarnings{}

	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes dynamic client: %w", err)
	}
	return client, nil
}

// podsGVR is the fixed core/v1 Pod resource, which discovery does not need to map.
var podsGVR = schema.GroupVersionResource{Version: "v1", Resource: "pods"}

// attributePods lists live Pods across namespace (empty means cluster-wide,
// matching a cluster-scoped root's search) and, for each view, sets PodStats
// from the pods whose owner-reference chain reaches its root UID. A listing
// failure degrades to a warning: pod stats are an enrichment, not the result.
func attributePods(
	ctx context.Context, dyn dynamic.Interface, mapper meta.RESTMapper, namespace string, views []workload.View,
) error {
	list, err := dyn.Resource(podsGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list pods for pod stats: %w", err)
	}

	pods := make([]corev1.Pod, 0, len(list.Items))
	for i := range list.Items {
		var pod corev1.Pod
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[i].Object, &pod); err != nil {
			return fmt.Errorf("decode pod %s: %w", list.Items[i].GetName(), err)
		}
		pods = append(pods, pod)
	}

	attributor := workload.NewPodAttributor(dyn, mapper)
	for i := range views {
		views[i].PodStats = attributor.Attribute(ctx, pods, apitypes.UID(views[i].UID))
	}
	return nil
}

// outputFormat reads the persistent -o flag off the root command.
func outputFormat(cmd *cobra.Command) generator.Output {
	flag := cmd.Root().PersistentFlags().Lookup(flagOutput)
	if flag == nil {
		return generator.OutputTable
	}
	return generator.Output(flag.Value.String())
}
