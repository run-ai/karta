// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var _ = Describe("Recording", func() {
	// Metadata, the ordered STATE states, and a performed action all survive the file.
	It("round-trips through writeRecording and loadRecording", func() {
		rec := Recording{
			SchemaVersion: schemaVersion,
			Operator:      "batch-job",
			KartaName:     "batch-job-v1",
			Flow:          "resumed",
			Want:          string(v1alpha1.CompletedStatus),
			Result:        Result{Failures: []string{"watch lost its position"}},
			Events: []Event{
				{Kind: EventState, State: "Suspended", ResourceVersion: "101", Object: map[string]interface{}{
					"kind": "Job", "metadata": map[string]interface{}{"name": "j"},
					"status": map[string]interface{}{"active": float64(0)},
				}},
				{Kind: EventAction, Action: &RecordedAction{
					Name: "Resume",
					Operation: Operation{
						Verb: "PATCH", PatchType: "application/merge-patch+json",
						Payload: map[string]interface{}{"spec": map[string]interface{}{"suspend": false}},
					},
				}},
				{Kind: EventState, State: "Completed", StaleObservedGeneration: true, Object: map[string]interface{}{
					"kind": "Job", "metadata": map[string]interface{}{"name": "j"},
					"status": map[string]interface{}{"active": float64(0), "succeeded": float64(1)},
				}},
			},
		}

		path := filepath.Join(GinkgoT().TempDir(), "op", "v1", "batch-job-v1", "resumed.yaml")
		Expect(writeRecording(path, rec)).To(Succeed())
		got, err := loadRecording(path)
		Expect(err).NotTo(HaveOccurred())

		Expect(got.Operator).To(Equal("batch-job"))
		Expect(got.Flow).To(Equal("resumed"))
		Expect(got.Events).To(HaveLen(3))
		Expect(got.states()).To(Equal([]string{"Suspended", "Completed"}))

		act := got.Events[1].Action
		Expect(act).NotTo(BeNil())
		Expect(act.Name).To(Equal("Resume"))
		Expect(act.Operation.Verb).To(Equal("PATCH"))
		Expect(act.Operation.Payload["spec"].(map[string]interface{})["suspend"]).To(Equal(false))

		Expect(got.Events[0].StaleObservedGeneration).To(BeFalse())
		Expect(got.Events[2].StaleObservedGeneration).To(BeTrue())
		Expect(got.Result.Succeeded).To(BeFalse())
		Expect(got.Result.Failures).To(Equal([]string{"watch lost its position"}))
		Expect(got.Events[0].ResourceVersion).To(Equal("101"))
	})

	It("walks the STATE events with the Reader, skipping ACTION events", func() {
		rec := Recording{Events: []Event{
			{Kind: EventState, State: "Initializing", Object: map[string]interface{}{"status": map[string]interface{}{"active": float64(1)}}},
			{Kind: EventAction, Action: &RecordedAction{Name: "Scale"}},
			{Kind: EventState, State: "Running", Object: map[string]interface{}{"status": map[string]interface{}{"ready": float64(1)}}},
		}}

		r := newReader(rec)
		var states []string
		for r.Next() {
			states = append(states, r.State())
		}
		Expect(states).To(Equal([]string{"Initializing", "Running"}))

		r2 := newReader(rec)
		r2.Next()
		r2.Next()
		ready, _, _ := unstructured.NestedFloat64(r2.Object().Object, "status", "ready")
		Expect(ready).To(Equal(float64(1)))
	})
})
