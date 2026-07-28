// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package kartas

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// GrovePodCliqueSet returns the built-in Karta for the Grove PodCliqueSet
// workload (grove.io/v1alpha1).
func GrovePodCliqueSet() *v1alpha1.Karta {
	// standaloneCliques selects only the PodClique templates that are NOT referenced by
	// any podCliqueScalingGroups[].cliqueNames. In Grove, .spec.template.cliques is the
	// shared master list of all clique templates; cliques named by a scaling group are
	// owned by that PodCliqueScalingGroup, not directly by the PodCliqueSet. Every clique
	// path derives from this expression so the instance, scale, and spec arrays stay
	// aligned on the same standalone cliques.
	standaloneCliques := `.spec.template as $t | $t.cliques[] | ` +
		`select(.name as $n | ([$t.podCliqueScalingGroups[]?.cliqueNames[]?] | index($n) | not))`
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "grove-io-podcliqueset-v1alpha1"},
		Spec: v1alpha1.KartaSpec{
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "podcliqueset",
					Kind: &v1alpha1.GroupVersionKind{Group: "grove.io", Version: "v1alpha1", Kind: "PodCliqueSet"},
					// Grove PCS has no aggregate phase field. The reliable signal for
					// running/initializing is replica counts (availableReplicas vs
					// spec.replicas). TopologyLevelsUnavailable is the only PCS-level
					// failure condition (TAS-specific); broader failures surface on
					// child PodClique/Pod resources.
					StatusDefinition: &v1alpha1.StatusDefinition{
						ConditionsDefinition: &v1alpha1.ConditionsDefinition{
							Path:             ".status.conditions",
							TypeFieldName:    "type",
							StatusFieldName:  "status",
							ReasonFieldName:  ptr.To("reason"),
							MessageFieldName: ptr.To("message"),
						},
						StatusMappings: v1alpha1.StatusMappings{
							Failed: []v1alpha1.StatusMatcher{{ByConditions: []v1alpha1.ExpectedCondition{{Type: "TopologyLevelsUnavailable", Status: ptr.To("True")}}}},
							Running: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								// Matches when all desired replicas are available (including the
								// vacuous replicas=0 case, same as k8s Deployment Available=True).
								Expression:     "(.status.availableReplicas // 0) >= (.spec.replicas // 0)",
								ExpectedResult: "true",
							}}},
							Initializing: []v1alpha1.StatusMatcher{{ByExpression: &v1alpha1.ExpressionMatcher{
								Expression:     "(.spec.replicas // 0) > 0 and (.status.availableReplicas // 0) < (.spec.replicas // 0)",
								ExpectedResult: "true",
							}}},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					// Standalone PodCliques: entries under .spec.template.cliques NOT referenced by
					// any .spec.template.podCliqueScalingGroups[].cliqueNames. PodCliques inside a
					// scaling group are owned by the PodCliqueScalingGroup and reached via the
					// `scalinggroup` child below.
					// TODO(follow-up): scaling-group member cliques' pod specs are not extracted
					// here; `scalinggroup` has no SpecDefinition, so per-clique spec/scale for
					// cliques inside a scaling group needs a dedicated component.
					{
						Name:     "clique",
						Kind:     &v1alpha1.GroupVersionKind{Group: "grove.io", Version: "v1alpha1", Kind: "PodClique"},
						OwnerRef: ptr.To("podcliqueset"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							// These paths derive from standaloneCliques, whose cross-reference filter
							// needs a `.spec.template as $t` binding. A jq expression with a binding is
							// not an assignable path, so these are read-only projections: pod-spec reads
							// work, but mutation is not supported for standalone-clique pods. Removing
							// the binding would drop the scaling-group filtering, so this is a
							// deliberate trade-off, not an oversight.
							FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
								SchedulerNamePath:     ptr.To(standaloneCliques + " | .spec.podSpec.schedulerName"),
								LabelsPath:            ptr.To(standaloneCliques + " | .labels"),
								AnnotationsPath:       ptr.To(standaloneCliques + " | .annotations"),
								ContainersPath:        ptr.To(standaloneCliques + " | .spec.podSpec.containers"),
								ResourceClaimsPath:    ptr.To(standaloneCliques + " | .spec.podSpec.resourceClaims"),
								PriorityClassNamePath: ptr.To(standaloneCliques + " | .spec.podSpec.priorityClassName"),
								PodAffinityPath:       ptr.To(standaloneCliques + " | .spec.podSpec.affinity.podAffinity"),
								NodeAffinityPath:      ptr.To(standaloneCliques + " | .spec.podSpec.affinity.nodeAffinity"),
							},
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							// A standalone clique's pods = PCS.replicas * clique.replicas.
							ReplicasPath:    ptr.To(".spec.replicas as $r | " + standaloneCliques + " | (($r // 1) * (.spec.replicas // 1))"),
							MinReplicasPath: ptr.To(standaloneCliques + " | .spec.autoScalingConfig.minReplicas"),
							MaxReplicasPath: ptr.To(standaloneCliques + " | .spec.autoScalingConfig.maxReplicas"),
						},
						InstanceIdPath: ptr.To(standaloneCliques + " | .name"),
						PodSelector: &v1alpha1.PodSelector{
							ComponentInstanceSelector: &v1alpha1.ComponentInstanceSelector{
								IdPath: `.metadata.labels["grove.io/podclique"]`,
							},
							ReplicaSelector: &v1alpha1.ReplicaSelector{
								KeyPath: `.metadata.labels["grove.io/podcliqueset-replica-index"]`,
							},
						},
					},
					// PodCliqueScalingGroups (each entry under .spec.template.podCliqueScalingGroups
					// becomes a PodCliqueScalingGroup CR per PCS replica).
					{
						Name:     "scalinggroup",
						Kind:     &v1alpha1.GroupVersionKind{Group: "grove.io", Version: "v1alpha1", Kind: "PodCliqueScalingGroup"},
						OwnerRef: ptr.To("podcliqueset"),
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							ReplicasPath:    ptr.To("(.spec.replicas // 1) * (.spec.template.podCliqueScalingGroups[].replicas // 1)"),
							MinReplicasPath: ptr.To(".spec.template.podCliqueScalingGroups[].scaleConfig.minReplicas"),
							MaxReplicasPath: ptr.To(".spec.template.podCliqueScalingGroups[].scaleConfig.maxReplicas"),
						},
						InstanceIdPath: ptr.To(".spec.template.podCliqueScalingGroups[].name"),
						PodSelector: &v1alpha1.PodSelector{
							ComponentInstanceSelector: &v1alpha1.ComponentInstanceSelector{
								IdPath: `.metadata.labels["grove.io/podcliquescalinggroup"]`,
							},
							ReplicaSelector: &v1alpha1.ReplicaSelector{
								KeyPath: `.metadata.labels["grove.io/podcliquescalinggroup-replica-index"]`,
							},
						},
					},
				},
				AdditionalChildKinds: []v1alpha1.GroupVersionKind{
					// PodCliques owned indirectly via PodCliqueScalingGroups also live in the tree.
					{Group: "grove.io", Version: "v1alpha1", Kind: "PodClique"},
					// PodCliqueScalingGroup must also appear here (even though it is a childComponent
					// above) so the external-workload-integrator's top-owner walker recognises it as
					// part of this workload and continues past it to reach the PCS root. Without this,
					// scaling-group pods get a top-owner of their own PodClique CR and form a separate workload.
					{Group: "grove.io", Version: "v1alpha1", Kind: "PodCliqueScalingGroup"},
					// Gang-scheduling primitive emitted by Grove for each PCS replica.
					{Group: "scheduler.grove.io", Version: "v1alpha1", Kind: "PodGang"},
				},
			},
			// Each PodCliqueSet replica is a gang: every pod (from standalone cliques and
			// scaling groups) sharing the same PCS name + replica index must be co-scheduled.
			Instructions: v1alpha1.OptimizationInstructions{
				GangScheduling: &v1alpha1.GangSchedulingInstruction{
					PodGroups: []v1alpha1.PodGroupDefinition{{
						Name: "podcliqueset-replica",
						Members: []v1alpha1.PodGroupMemberDefinition{
							{
								ComponentName: "clique",
								GroupByKeyPaths: []string{
									`.metadata.labels["app.kubernetes.io/part-of"]`,
									`.metadata.labels["grove.io/podcliqueset-replica-index"]`,
								},
							},
							{
								ComponentName: "scalinggroup",
								GroupByKeyPaths: []string{
									`.metadata.labels["app.kubernetes.io/part-of"]`,
									`.metadata.labels["grove.io/podcliqueset-replica-index"]`,
								},
							},
						},
					}},
				},
			},
		},
	}
}
