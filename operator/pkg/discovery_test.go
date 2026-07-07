// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// fakeDiscovery overrides only ServerGroupsAndResources; the embedded nil
// interface satisfies the rest of discovery.DiscoveryInterface (unused here).
type fakeDiscovery struct {
	discovery.DiscoveryInterface
	lists []*metav1.APIResourceList
	err   error
}

func (f *fakeDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, f.lists, f.err
}

func apiResourceList(groupVersion string, resources ...metav1.APIResource) *metav1.APIResourceList {
	return &metav1.APIResourceList{GroupVersion: groupVersion, APIResources: resources}
}

var _ = Describe("discoverNativeGVKs", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	reader := func(crds ...client.Object) client.Reader {
		return fake.NewClientBuilder().WithScheme(buildScheme()).WithObjects(crds...).Build()
	}

	It("keeps built-in list+watch kinds and drops subresources, virtual, and CRD-backed kinds", func() {
		dc := &fakeDiscovery{lists: []*metav1.APIResourceList{
			apiResourceList("apps/v1",
				metav1.APIResource{Name: "deployments", Kind: "Deployment", Verbs: metav1.Verbs{"get", "list", "watch"}},
				metav1.APIResource{Name: "deployments/status", Kind: "Deployment", Verbs: metav1.Verbs{"get", "update"}},
			),
			apiResourceList("authorization.k8s.io/v1",
				metav1.APIResource{Name: "subjectaccessreviews", Kind: "SubjectAccessReview", Verbs: metav1.Verbs{"create"}},
			),
			apiResourceList("test.run.ai/v1",
				metav1.APIResource{Name: "foos", Kind: "Foo", Verbs: metav1.Verbs{"get", "list", "watch"}},
			),
		}}

		native, err := discoverNativeGVKs(ctx, dc,
			reader(newCRD("foos.test.run.ai", "test.run.ai", "Foo", "v1")), logr.Discard())

		Expect(err).NotTo(HaveOccurred())
		Expect(native).To(HaveKey(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}))
		Expect(native).NotTo(HaveKey(schema.GroupVersionKind{Group: "authorization.k8s.io", Version: "v1", Kind: "SubjectAccessReview"}))
		Expect(native).NotTo(HaveKey(schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}))
		Expect(native).To(HaveLen(1))
	})

	It("tolerates a stale-only group discovery failure and uses the reachable groups", func() {
		staleGV := schema.GroupVersion{Group: "custom.metrics.k8s.io", Version: "v1beta1"}
		dc := &fakeDiscovery{
			lists: []*metav1.APIResourceList{apiResourceList("apps/v1",
				metav1.APIResource{Name: "deployments", Kind: "Deployment", Verbs: metav1.Verbs{"get", "list", "watch"}},
			)},
			err: &discovery.ErrGroupDiscoveryFailed{Groups: map[schema.GroupVersion]error{
				staleGV: discovery.StaleGroupVersionError{},
			}},
		}

		native, err := discoverNativeGVKs(ctx, dc, reader(), logr.Discard())

		Expect(err).NotTo(HaveOccurred())
		Expect(native).To(HaveKey(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}))
	})

	It("fails on a non-stale (legacy per-group) discovery failure", func() {
		appsGV := schema.GroupVersion{Group: "apps", Version: "v1"}
		dc := &fakeDiscovery{err: &discovery.ErrGroupDiscoveryFailed{Groups: map[schema.GroupVersion]error{
			appsGV: errors.New("boom"),
		}}}

		native, err := discoverNativeGVKs(ctx, dc, reader(), logr.Discard())

		Expect(err).To(HaveOccurred())
		Expect(native).To(BeNil())
	})

	It("fails on a generic discovery error", func() {
		dc := &fakeDiscovery{err: errors.New("connection refused")}

		native, err := discoverNativeGVKs(ctx, dc, reader(), logr.Discard())

		Expect(err).To(HaveOccurred())
		Expect(native).To(BeNil())
	})
})
