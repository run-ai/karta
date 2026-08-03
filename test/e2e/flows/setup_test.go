// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package flows

import (
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// installKarta applies a Karta definition and waits for it to reconcile Ready. The definition is deleted
// after the spec tree that installed it. This is test setup - the recorder never installs Karta.
func installKarta(ctx context.Context, kartaFile, kartaName string) {
	karta := &kartav1alpha1.Karta{}
	Expect(yaml.Unmarshal(readE2E(kartaFile), karta)).To(Succeed())
	if karta.GetName() == "" {
		karta.SetName(kartaName) // some bundled definitions omit metadata.name
	}
	Expect(k8sClient.Create(ctx, karta)).To(Succeed())
	DeferCleanup(func(ctx SpecContext) { _ = k8sClient.Delete(ctx, karta) })

	Eventually(func(g Gomega) {
		got := &kartav1alpha1.Karta{}
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: kartaName}, got)).To(Succeed())
		g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, "Ready")).To(BeTrue(), "Ready")
	}, time.Minute, 2*time.Second).Should(Succeed())
}

// readE2E reads a path relative to test/e2e (the flows package runs from test/e2e/flows).
func readE2E(path string) []byte {
	b, err := os.ReadFile(filepath.Join("..", path))
	Expect(err).NotTo(HaveOccurred(), "read %s", path)
	return b
}
