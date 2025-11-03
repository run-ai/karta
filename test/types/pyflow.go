// +kubebuilder:object:generate=true

package types

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"

	"k8s.io/utils/ptr"
)

// PyFlow represents a PyTorch-like training job with hardcoded component fields
// Single pod template concept, multiple components via separate fields
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PyFlow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PyFlowSpec   `json:"spec,omitempty"`
	Status            PyFlowStatus `json:"status,omitempty"`
}

type PyFlowSpec struct {
	// Master defines the master component
	Master ReplicaSpec `json:"master,omitempty"`

	// Worker defines the worker component
	Worker ReplicaSpec `json:"worker,omitempty"`
}

type ReplicaSpec struct {
	// Replicas is the desired number of replicas for this role
	Replicas *int32 `json:"replicas,omitempty"`

	// MinReplicas is the minimum number of replicas for this role
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the maximum number of replicas for this role
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`

	// Template is the pod template for this role
	Template corev1.PodTemplateSpec `json:"template"`
}

type PyFlowStatus struct {
	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// PyFlowRI returns a ResourceInterface for PyFlow
// Models simple structure: hardcoded fields, multiple components (master + workers)
func PyFlowRI() *v1alpha1.ResourceInterface {
	return &v1alpha1.ResourceInterface{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pyflow",
		},
		Spec: v1alpha1.ResourceInterfaceSpec{
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "pyflow",
					Kind: &v1alpha1.GroupVersionKind{
						Group:   "jobs.example.com",
						Version: "v1",
						Kind:    "PyFlow",
					},
					StatusDefinition: &v1alpha1.StatusDefinition{
						ConditionsDefinition: &v1alpha1.ConditionsDefinition{
							Path:            ".status.conditions",
							TypeFieldName:   "type",
							StatusFieldName: "status",
						},
						StatusMappings: v1alpha1.StatusMappings{
							Running: []v1alpha1.StatusMatcher{
								{
									ByConditions: []v1alpha1.ExpectedCondition{
										{
											Type:   "Running",
											Status: "True",
										},
									},
								},
							},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "master",
						OwnerRef: ptr.To("pyflow"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodTemplateSpecPath: ptr.To(".spec.master.template"),
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							ReplicasPath: ptr.To(".spec.master.replicas"),
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								KeyPath: ".metadata.labels.role",
								Value:   ptr.To("master"),
							},
						},
					},
					{
						Name:     "worker",
						OwnerRef: ptr.To("pyflow"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodTemplateSpecPath: ptr.To(".spec.worker.template"),
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							MinReplicasPath: ptr.To(".spec.worker.minReplicas"),
							MaxReplicasPath: ptr.To(".spec.worker.maxReplicas"),
						},
						PodSelector: &v1alpha1.PodSelector{
							ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
								KeyPath: ".metadata.labels.role",
								Value:   ptr.To("worker"),
							},
						},
					},
				},
			},
		},
	}
}

// NewPyFlowObject creates a test instance of PyFlow
// Simple job structure with hardcoded master and worker fields
func NewPyFlowObject() *PyFlow {
	return &PyFlow{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "jobs.example.com/v1",
			Kind:       "PyFlow",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pyflow-example",
			Namespace: "default",
			Labels: map[string]string{
				"app":  "pyflow",
				"type": "ml-training",
			},
		},
		Spec: PyFlowSpec{
			Master: ReplicaSpec{
				Replicas: ptr.To(int32(1)),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":   "pyflow",
							"role":  "master",
							"index": "0",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "trainer",
								Image: "tensorflow/tensorflow:2.8.0-gpu",
								Command: []string{
									"python",
									"/app/train.py",
									"--backend=nccl",
								},
								Env: []corev1.EnvVar{
									{
										Name:  "MASTER_PORT",
										Value: "23456",
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("1"),
										corev1.ResourceMemory: resource.MustParse("2Gi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("2"),
										corev1.ResourceMemory: resource.MustParse("4Gi"),
										"nvidia.com/gpu":      resource.MustParse("1"),
									},
								},
							},
						},
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
			Worker: ReplicaSpec{
				MinReplicas: ptr.To(int32(1)),
				MaxReplicas: ptr.To(int32(5)),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":   "pyflow",
							"role":  "worker",
							"index": "1",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "trainer",
								Image: "tensorflow/tensorflow:2.8.0-gpu",
								Command: []string{
									"python",
									"/app/train.py",
									"--backend=nccl",
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("2"),
										corev1.ResourceMemory: resource.MustParse("4Gi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("4"),
										corev1.ResourceMemory: resource.MustParse("8Gi"),
										"nvidia.com/gpu":      resource.MustParse("1"),
									},
								},
							},
						},
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
		},
		Status: PyFlowStatus{
			Conditions: []metav1.Condition{
				{
					Type:   "Running",
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
}
