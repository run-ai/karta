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
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/run-ai/karta/pkg/catalog"
)

var kartasResource = schema.GroupResource{Group: "run.ai", Resource: "kartas"}

// emptyConfigGetter is a RESTClientGetter with nothing configured, which is what
// running without a kubeconfig looks like to FetchCluster.
func emptyConfigGetter() genericclioptions.RESTClientGetter {
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
	var warn *bytes.Buffer

	BeforeEach(func() {
		warn = &bytes.Buffer{}
	})

	It("stays silent for a wrapped empty-config error from the real FetchCluster", func() {
		_, err := FetchCluster(context.Background(), emptyConfigGetter())
		Expect(err).To(HaveOccurred())

		// The wrapping is exactly what defeats clientcmd.IsEmptyConfig, so classify
		// has to unwrap before it can recognise the offline case.
		Expect(clientcmd.IsEmptyConfig(err)).To(BeFalse())
		Expect(clientcmd.IsEmptyConfig(errors.Unwrap(err))).To(BeTrue())

		classify(err, warn)
		Expect(warn.String()).To(BeEmpty())
	})

	It("warns when an explicit kubeconfig path does not exist", func() {
		missing := filepath.Join(GinkgoT().TempDir(), "missing-kubeconfig")
		flags := genericclioptions.NewConfigFlags(false)
		flags.KubeConfig = &missing

		_, err := FetchCluster(context.Background(), flags)
		Expect(err).To(HaveOccurred())

		classify(err, warn)
		Expect(warn.String()).To(HavePrefix("warning:"))
		Expect(warn.String()).To(ContainSubstring("missing-kubeconfig"))
	})

	It("stays silent when the Karta CRD is not installed", func() {
		err := fmt.Errorf("list Karta definitions: %w", apierrors.NewNotFound(kartasResource, ""))

		classify(err, warn)
		Expect(warn.String()).To(BeEmpty())
	})

	It("warns and names the resource when listing is forbidden", func() {
		err := fmt.Errorf("list Karta definitions: %w",
			apierrors.NewForbidden(kartasResource, "", errors.New("RBAC: access denied")))

		classify(err, warn)
		Expect(warn.String()).To(HavePrefix("warning:"))
		Expect(warn.String()).To(ContainSubstring("kartas.run.ai"))
		Expect(warn.String()).To(ContainSubstring("list"))
	})

	It("warns with the cause for any other failure", func() {
		err := fmt.Errorf("list Karta definitions: %w", errors.New("connection refused"))

		classify(err, warn)
		Expect(warn.String()).To(HavePrefix("warning:"))
		Expect(warn.String()).To(ContainSubstring("connection refused"))
	})

	It("never leaks apimachinery no-match wording", func() {
		err := fmt.Errorf("list Karta definitions: %w", apierrors.NewNotFound(kartasResource, ""))

		classify(err, warn)
		Expect(warn.String()).NotTo(ContainSubstring("no matches for kind"))
	})
})

var _ = Describe("Load", func() {
	var (
		ctx  context.Context
		warn *bytes.Buffer
	)

	BeforeEach(func() {
		ctx = context.Background()
		warn = &bytes.Buffer{}
	})

	It("falls back to the built-in catalog when no cluster is reachable", func() {
		resolver := Load(ctx, emptyConfigGetter(), warn)

		Expect(warn.String()).To(BeEmpty())

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

		resolver := Load(ctx, serverConfigGetter(server), warn)

		Expect(warn.String()).To(BeEmpty())
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

			resolver := Load(ctx, serverConfigGetter(server), warn)

			Expect(warn.String()).To(Equal(want))

			def, err := resolver.Resolve(deploymentGVK)
			Expect(err).NotTo(HaveOccurred())
			Expect(def.Karta.Name).To(Equal("zzz-deployment"))
		},
		Entry("two claimants",
			[]string{"aaa-deployment", "zzz-deployment"},
			"warning: 2 cluster Karta definitions claim apps/v1, Kind=Deployment; "+
				`using "zzz-deployment" and ignoring "aaa-deployment"`+"\n",
		),
		Entry("four claimants",
			[]string{"zzz-deployment", "aaa-deployment", "ccc-deployment", "bbb-deployment"},
			"warning: 4 cluster Karta definitions claim apps/v1, Kind=Deployment; "+
				`using "zzz-deployment" and ignoring "aaa-deployment", "bbb-deployment", "ccc-deployment"`+"\n",
		),
	)
})
