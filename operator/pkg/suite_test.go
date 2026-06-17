// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"testing"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Karta Operator Suite")
}

// buildScheme builds a runtime.Scheme populated with the API types this
// operator interacts with. Used by every test that creates a fake client.
func buildScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	Expect(kartav1alpha1.AddToScheme(s)).To(Succeed())
	Expect(apiextensionsv1.AddToScheme(s)).To(Succeed())
	return s
}

// newReconciler wraps NewReconciler so test files don't need to repeat the
// recorder on every call. Events are buffered and silently discarded unless
// the test explicitly reads from the recorder channel.
func newReconciler(k8s client.WithWatch) *Reconciler {
	return NewReconciler(k8s, record.NewFakeRecorder(64))
}
