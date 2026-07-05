// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package e2e runs the Karta end-to-end suite against a real cluster. The
// cluster and its operators are provisioned out of band by hack/e2e/up.sh; these
// tests only connect to the current kube context and exercise behaviour. The
// suite lives in its own Go module so the controller-runtime and client test
// dependencies stay out of the published karta library.
package e2e

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var (
	ctx       = context.Background()
	k8sClient client.Client
)

// TestE2E is the Go test entry point; it hands control to Ginkgo, which runs the
// specs defined across this package against the cluster in the current context.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Karta E2E Suite")
}

var _ = BeforeSuite(func() {
	scheme := runtime.NewScheme()
	utilruntime.Must(kartav1alpha1.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	cfg := ctrl.GetConfigOrDie() // current kube context, provisioned by hack/e2e/up.sh
	var err error
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
})
