// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package integration

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"strings"
	"time"

	"github.com/run-ai/karta/operator/pkg"
	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Reconciler (envtest)", func() {
	getKarta := func(k *kartav1alpha1.Karta) *kartav1alpha1.Karta {
		out := &kartav1alpha1.Karta{}
		Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(k), out)).To(Succeed())
		return out
	}

	It("sets Validated=False with the real validator error for an invalid Karta", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		k := newKarta("envtest-invalid", &gvk) // no StatusDefinition → invalid
		Expect(k8sClient.Create(testCtx, k)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(testCtx, k) })

		Eventually(func(g Gomega) {
			c, ok := findCondition(getKarta(k).Status.Conditions, kartav1alpha1.ConditionValidated)
			g.Expect(ok).To(BeTrue())
			g.Expect(c.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(c.Reason).To(Equal(pkg.ReasonValidationFailed))
			g.Expect(c.Message).NotTo(BeEmpty())
			g.Expect(c.Message).NotTo(Equal("Karta validation failed"),
				"expected the real validator error, not the generic fallback")
		}, eventuallyTimeout, eventuallyInterval).Should(Succeed())
	})

	It("sets CRDExists=False and Ready=False when the referenced CRD is missing", func() {
		gvk := schema.GroupVersionKind{Group: "absent.run.ai", Version: "v1", Kind: "Missing"}
		k := newValidKarta("envtest-no-crd", &gvk)
		Expect(k8sClient.Create(testCtx, k)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(testCtx, k) })

		Eventually(func(g Gomega) {
			conds := getKarta(k).Status.Conditions
			crd, ok := findCondition(conds, kartav1alpha1.ConditionCRDExists)
			g.Expect(ok).To(BeTrue())
			g.Expect(crd.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(crd.Message).To(ContainSubstring("absent.run.ai"))
			ready, ok := findCondition(conds, kartav1alpha1.ConditionReady)
			g.Expect(ok).To(BeTrue())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		}, eventuallyTimeout, eventuallyInterval).Should(Succeed())
	})

	It("sets Ready=True and stamps index labels when the CRD exists", func() {
		crd := buildCRD("widgets.test.run.ai", "test.run.ai", "Widget", "v1")
		Expect(k8sClient.Create(testCtx, crd)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(testCtx, crd) })

		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Widget"}
		k := newValidKarta("envtest-ready", &gvk)
		Expect(k8sClient.Create(testCtx, k)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(testCtx, k) })

		Eventually(func(g Gomega) {
			got := getKarta(k)
			validated, ok := findCondition(got.Status.Conditions, kartav1alpha1.ConditionValidated)
			g.Expect(ok).To(BeTrue())
			g.Expect(validated.Status).To(Equal(metav1.ConditionTrue))
			crdExists, ok := findCondition(got.Status.Conditions, kartav1alpha1.ConditionCRDExists)
			g.Expect(ok).To(BeTrue())
			g.Expect(crdExists.Status).To(Equal(metav1.ConditionTrue))
			ready, ok := findCondition(got.Status.Conditions, kartav1alpha1.ConditionReady)
			g.Expect(ok).To(BeTrue())
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(got.Labels[kartav1alpha1.LabelRootGroup]).To(Equal(gvk.Group))
			g.Expect(got.Labels[kartav1alpha1.LabelRootVersion]).To(Equal(gvk.Version))
			g.Expect(got.Labels[kartav1alpha1.LabelRootKind]).To(Equal(gvk.Kind))
		}, eventuallyTimeout, eventuallyInterval).Should(Succeed())
	})

	It("preserves a foreign condition written by another controller", func() {
		gvk := schema.GroupVersionKind{Group: "absent.run.ai", Version: "v1", Kind: "Missing"}
		k := newValidKarta("envtest-foreign", &gvk)
		Expect(k8sClient.Create(testCtx, k)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(testCtx, k) })

		Eventually(func(g Gomega) {
			_, ok := findCondition(getKarta(k).Status.Conditions, kartav1alpha1.ConditionReady)
			g.Expect(ok).To(BeTrue())
		}, eventuallyTimeout, eventuallyInterval).Should(Succeed())

		Eventually(func() error {
			got := getKarta(k)
			got.Status.Conditions = append(got.Status.Conditions, metav1.Condition{
				Type: "RBACReady", Status: metav1.ConditionTrue, Reason: "EWI",
				LastTransitionTime: metav1.Now(),
			})
			return k8sClient.Status().Update(testCtx, got)
		}, eventuallyTimeout, eventuallyInterval).Should(Succeed())

		Eventually(func() error {
			got := getKarta(k)
			got.Spec.StructureDefinition.RootComponent.Name = "renamed-root"
			return k8sClient.Update(testCtx, got)
		}, eventuallyTimeout, eventuallyInterval).Should(Succeed())

		Consistently(func(g Gomega) {
			rbac, ok := findCondition(getKarta(k).Status.Conditions, "RBACReady")
			g.Expect(ok).To(BeTrue())
			g.Expect(rbac.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(rbac.Reason).To(Equal("EWI"))
		}, 2*time.Second, eventuallyInterval).Should(Succeed())
	})
})

func newKarta(name string, gvk *schema.GroupVersionKind) *kartav1alpha1.Karta {
	k := &kartav1alpha1.Karta{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	if gvk != nil {
		k.Spec.StructureDefinition.RootComponent.Name = name + "-root"
		k.Spec.StructureDefinition.RootComponent.Kind = &kartav1alpha1.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		}
	}
	return k
}

func newValidKarta(name string, gvk *schema.GroupVersionKind) *kartav1alpha1.Karta {
	k := newKarta(name, gvk)
	if gvk != nil {
		k.Spec.StructureDefinition.RootComponent.StatusDefinition = &kartav1alpha1.StatusDefinition{
			StatusMappings: kartav1alpha1.StatusMappings{},
		}
	}
	return k
}

func findCondition(conds []metav1.Condition, t kartav1alpha1.ConditionType) (metav1.Condition, bool) {
	for _, c := range conds {
		if c.Type == string(t) {
			return c, true
		}
	}
	return metav1.Condition{}, false
}

func buildCRD(name, group, kind, version string) *apiextensionsv1.CustomResourceDefinition {
	preserve := true
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   kind,
				Plural: strings.ToLower(kind) + "s",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    version,
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type:                   "object",
						XPreserveUnknownFields: &preserve,
					},
				},
			}},
		},
	}
}
