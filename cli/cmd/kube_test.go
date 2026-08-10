// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func contextWithNamespace(ns string) clientcmd.ClientConfig {
	cfg := clientcmdapi.NewConfig()
	cfg.CurrentContext = "ctx"
	cfg.Clusters["cluster"] = &clientcmdapi.Cluster{Server: "https://localhost:6443"}
	cfg.Contexts["ctx"] = &clientcmdapi.Context{Cluster: "cluster", Namespace: ns}
	return clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{})
}

var _ = DescribeTable("ResolvedNamespace",
	func(rcg genericclioptions.RESTClientGetter, wantNS string, wantExplicit bool) {
		ns, explicit, err := ResolvedNamespace(rcg)
		Expect(err).NotTo(HaveOccurred())
		Expect(ns).To(Equal(wantNS))
		Expect(explicit).To(Equal(wantExplicit))
	},
	Entry("explicit flag wins",
		genericclioptions.NewTestConfigFlags().
			WithClientConfig(contextWithNamespace("ctx-ns")).
			WithNamespace("ml-team"),
		"ml-team", true,
	),
	Entry("kubeconfig context namespace",
		genericclioptions.NewTestConfigFlags().WithClientConfig(contextWithNamespace("ctx-ns")),
		"ctx-ns", false,
	),
	Entry("default fallback",
		genericclioptions.NewTestConfigFlags().WithClientConfig(contextWithNamespace("")),
		"default", false,
	),
)
