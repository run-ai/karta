// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package definitions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog"
)

var kartasResource = schema.GroupResource{Group: "run.ai", Resource: "kartas"}

// noKubeconfigGetter is a RESTClientGetter with nothing configured, which is what
// running without a kubeconfig looks like to FetchCluster.
func noKubeconfigGetter() genericclioptions.RESTClientGetter {
	return genericclioptions.NewTestConfigFlags().WithClientConfig(
		clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{},
			&clientcmd.ConfigOverrides{},
		),
	)
}

// serverConfigGetter points a RESTClientGetter at an in-process test server.
func serverConfigGetter(server *httptest.Server) genericclioptions.RESTClientGetter {
	cfg := clientcmdapi.NewConfig()
	cfg.CurrentContext = "test"
	cfg.Clusters["test"] = &clientcmdapi.Cluster{Server: server.URL}
	cfg.Contexts["test"] = &clientcmdapi.Context{Cluster: "test", AuthInfo: "test"}
	cfg.AuthInfos["test"] = &clientcmdapi.AuthInfo{}

	return genericclioptions.NewTestConfigFlags().
		WithClientConfig(clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{}))
}

// kartaListJSON renders the API response for a list of Kartas that all claim
// apps/v1 Deployment as their root component.
func kartaListJSON(names ...string) string {
	items := make([]string, 0, len(names))
	for _, name := range names {
		items = append(items, fmt.Sprintf(`{"apiVersion":"run.ai/v1alpha1","kind":"Karta",`+
			`"metadata":{"name":%q},`+
			`"spec":{"structureDefinition":{"rootComponent":{"name":"root",`+
			`"kind":{"group":"apps","version":"v1","kind":"Deployment"}}}}}`, name))
	}
	body := `{"apiVersion":"run.ai/v1alpha1","kind":"KartaList","metadata":{"resourceVersion":"1"},"items":[`
	for i, item := range items {
		if i > 0 {
			body += ","
		}
		body += item
	}
	return body + `]}`
}

var _ = Describe("classify", func() {
	It("stays silent for a wrapped empty-config error from the real FetchCluster", func() {
		_, err := FetchCluster(context.Background(), noKubeconfigGetter())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("kubernetes config"))
		Expect(err.Error()).To(ContainSubstring("no configuration has been provided"))

		// The wrapping is exactly what defeats clientcmd.IsEmptyConfig, so classify
		// has to unwrap before it can recognise the offline case.
		Expect(clientcmd.IsEmptyConfig(err)).To(BeFalse())
		Expect(clientcmd.IsEmptyConfig(errors.Unwrap(err))).To(BeTrue())

		_, ok := classify(err)
		Expect(ok).To(BeFalse())
	})

	It("warns when an explicit kubeconfig path does not exist", func() {
		missing := filepath.Join(GinkgoT().TempDir(), "missing-kubeconfig")
		flags := genericclioptions.NewConfigFlags(false)
		flags.KubeConfig = &missing

		_, err := FetchCluster(context.Background(), flags)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("kubernetes config"))
		Expect(err.Error()).To(ContainSubstring("missing-kubeconfig"))

		got, ok := classify(err)
		Expect(ok).To(BeTrue())
		Expect(got.Reason).To(Equal(ReasonUnreachable))
		Expect(got.Message).To(ContainSubstring("missing-kubeconfig"))
	})

	It("warns when the Karta CRD is not installed", func() {
		err := fmt.Errorf("list Karta definitions: %w", apierrors.NewNotFound(kartasResource, ""))

		got, ok := classify(err)
		Expect(ok).To(BeTrue())
		Expect(got.Reason).To(Equal(ReasonNoCRD))
		Expect(got.Message).To(ContainSubstring("kartas.run.ai"))
	})

	It("warns and names the resource when listing is forbidden", func() {
		err := fmt.Errorf("list Karta definitions: %w",
			apierrors.NewForbidden(kartasResource, "", errors.New("RBAC: access denied")))

		got, ok := classify(err)
		Expect(ok).To(BeTrue())
		Expect(got.Reason).To(Equal(ReasonForbidden))
		Expect(got.Message).To(ContainSubstring("kartas.run.ai"))
		Expect(got.Message).To(ContainSubstring("list"))
	})

	It("warns with the cause for any other failure", func() {
		err := fmt.Errorf("list Karta definitions: %w", errors.New("connection refused"))

		got, ok := classify(err)
		Expect(ok).To(BeTrue())
		Expect(got.Reason).To(Equal(ReasonUnreachable))
		Expect(got.Message).To(ContainSubstring("connection refused"))
	})

	It("never leaks apimachinery no-match wording", func() {
		err := fmt.Errorf("list Karta definitions: %w", apierrors.NewNotFound(kartasResource, ""))

		got, _ := classify(err)
		Expect(got.Message).NotTo(ContainSubstring("no matches for kind"))
	})
})

var _ = Describe("Load", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("falls back to the built-in catalog when no cluster is reachable", func() {
		resolver, warnings := Load(ctx, noKubeconfigGetter())

		Expect(warnings).To(BeEmpty())

		list := resolver.List()
		Expect(list).To(HaveLen(len(catalog.List())))
		for _, def := range list {
			Expect(def.Origin).To(Equal(OriginCommunity))
		}
	})

	It("merges cluster definitions and reads them cluster-scoped", func() {
		var requestPaths []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestPaths = append(requestPaths, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, kartaListJSON("cluster-deployment"))
		}))
		DeferCleanup(server.Close)

		resolver, warnings := Load(ctx, serverConfigGetter(server))

		Expect(warnings).To(BeEmpty())
		Expect(requestPaths).To(Equal([]string{"/apis/run.ai/v1alpha1/kartas"}))

		def, err := resolver.Resolve(deploymentGVK)
		Expect(err).NotTo(HaveOccurred())
		Expect(def.Karta.Name).To(Equal("cluster-deployment"))
		Expect(def.Origin).To(Equal(OriginCluster))
	})

	DescribeTable("warns once per cluster collision",
		func(names []string, want string) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, kartaListJSON(names...))
			}))
			DeferCleanup(server.Close)

			resolver, warnings := Load(ctx, serverConfigGetter(server))

			Expect(warnings).To(HaveLen(1))
			Expect(warnings[0].Reason).To(Equal(ReasonCollision))
			Expect(warnings[0].Message).To(Equal(want))

			_, err := resolver.Resolve(deploymentGVK)
			Expect(err).To(MatchError(ErrAmbiguous))
		},
		Entry("two claimants",
			[]string{"aaa-deployment", "zzz-deployment"},
			"2 cluster Karta definitions claim apps/v1, Kind=Deployment: "+
				`"aaa-deployment", "zzz-deployment"`,
		),
		Entry("four claimants",
			[]string{"zzz-deployment", "aaa-deployment", "ccc-deployment", "bbb-deployment"},
			"4 cluster Karta definitions claim apps/v1, Kind=Deployment: "+
				`"aaa-deployment", "bbb-deployment", "ccc-deployment", "zzz-deployment"`,
		),
	)
})

// newFakeDynamic builds a dynamic fake over a hand-rolled unstructured scheme.
// The Karta types must map to unstructured here: the object tracker builds the
// list result with meta.SetList, which cannot assign []*unstructured.Unstructured
// into the typed KartaList that v1alpha1.AddToScheme would register.
func newFakeDynamic(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(v1alpha1.GroupVersion.WithKind("Karta"), &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(v1alpha1.GroupVersion.WithKind("KartaList"), &unstructured.UnstructuredList{})

	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{GVR: "KartaList"},
		objects...,
	)
}

// loadFixture reads a "kubectl get -o yaml" style List document into the raw
// unstructured objects a dynamic client would hand back.
func loadFixture(name string) []runtime.Object {
	GinkgoHelper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred())

	var list unstructured.UnstructuredList
	Expect(utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096).Decode(&list)).To(Succeed())

	objects := make([]runtime.Object, 0, len(list.Items))
	for i := range list.Items {
		objects = append(objects, &list.Items[i])
	}
	return objects
}

var _ = Describe("listWithClient", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("converts the cluster payload into typed Kartas", func() {
		kartas, err := listWithClient(ctx, newFakeDynamic(loadFixture("kartas-list.yaml")...))
		Expect(err).NotTo(HaveOccurred())
		Expect(kartas).To(HaveLen(2))

		byName := map[string]*v1alpha1.Karta{}
		for _, k := range kartas {
			byName[k.Name] = k
		}
		Expect(byName).To(HaveKey("cluster-deployment"))
		Expect(byName).To(HaveKey("cluster-pod"))

		deployment := byName["cluster-deployment"]
		Expect(deployment.Kind).To(Equal("Karta"))
		Expect(deployment.APIVersion).To(Equal("run.ai/v1alpha1"))
		Expect(deployment.Generation).To(BeEquivalentTo(3))
		Expect(deployment.CreationTimestamp.IsZero()).To(BeFalse())
		Expect(deployment.Labels).To(HaveKeyWithValue("karta.run.ai/kind", "deployment"))

		root := deployment.Spec.StructureDefinition.RootComponent
		Expect(root.Kind).To(Equal(&v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}))
		Expect(root.SpecDefinition.PodTemplateSpecPath).To(HaveValue(Equal(".spec.template")))
		Expect(root.ScaleDefinition.ReplicasPath).To(HaveValue(Equal(".spec.replicas")))
		Expect(root.StatusDefinition.ConditionsDefinition.Path).To(Equal(".status.conditions"))
		Expect(root.StatusDefinition.StatusMappings.Running).To(HaveLen(1))
		Expect(root.StatusDefinition.StatusMappings.Running[0].ByConditions[0].Status).To(HaveValue(Equal("True")))
		Expect(root.StatusDefinition.StatusMappings.Failed[0].ByExpression.ExpectedResult).To(Equal("1"))
		Expect(root.SuspendDefinition.SuspendActions).To(Equal([]v1alpha1.SuspendAction{{Path: ".spec.replicas", Value: "0"}}))

		children := deployment.Spec.StructureDefinition.ChildComponents
		Expect(children).To(HaveLen(1))
		Expect(children[0].OwnerRef).To(HaveValue(Equal("deployment")))
		Expect(children[0].InstanceIdPath).To(HaveValue(Equal(".metadata.name")))
		Expect(children[0].PodSelector.ComponentTypeSelector.Value).To(HaveValue(Equal("worker")))
		Expect(children[0].PodSelector.ReplicaSelector.KeyPath).To(Equal(`.metadata.labels["replica-index"]`))

		Expect(deployment.Spec.StructureDefinition.AdditionalChildKinds).
			To(Equal([]v1alpha1.GroupVersionKind{{Version: "v1", Kind: "Service"}}))

		podGroup := deployment.Spec.Instructions.GangScheduling.PodGroup
		Expect(podGroup.Name).To(Equal("deployment-group"))
		Expect(podGroup.Topology.RequiredTopologyLevel).To(Equal("datacenter"))
		Expect(podGroup.SubGroups[0].Topology.PreferredTopologyLevel).To(Equal("rack"))

		Expect(deployment.Status.Conditions).To(HaveLen(1))
		Expect(deployment.Status.Conditions[0].Type).To(Equal("Validated"))
		Expect(deployment.Status.Conditions[0].LastTransitionTime.IsZero()).To(BeFalse())
		Expect(deployment.Status.Conditions[0].ObservedGeneration).To(BeEquivalentTo(3))

		pod := byName["cluster-pod"]
		Expect(pod.Spec.StructureDefinition.RootComponent.Kind.Group).To(BeEmpty())
		Expect(pod.Spec.StructureDefinition.RootComponent.SpecDefinition.MetadataPath).To(HaveValue(Equal(".metadata")))
	})

	It("issues a cluster-scoped list against the kartas resource", func() {
		client := newFakeDynamic(loadFixture("kartas-list.yaml")...)

		var recorded []k8stesting.ListAction
		client.PrependReactor("list", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
			recorded = append(recorded, action.(k8stesting.ListAction))
			return false, nil, nil
		})

		_, err := listWithClient(ctx, client)
		Expect(err).NotTo(HaveOccurred())

		Expect(recorded).To(HaveLen(1))
		Expect(recorded[0].GetResource()).To(Equal(GVR))
		Expect(recorded[0].GetNamespace()).To(BeEmpty())
		Expect(recorded[0].GetListRestrictions().Labels.Empty()).To(BeTrue())
		Expect(recorded[0].GetListRestrictions().Fields.Empty()).To(BeTrue())
	})

	It("returns no definitions and no error for an empty cluster", func() {
		kartas, err := listWithClient(ctx, newFakeDynamic())
		Expect(err).NotTo(HaveOccurred())
		Expect(kartas).To(BeEmpty())
	})
})
