// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package store

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// WorkloadRef identifies a workload object for metric labels.
type WorkloadRef struct {
	Namespace string
	Name      string
	Group     string
	Version   string
	Kind      string
}

// ComponentState holds the desired state of one component instance.
type ComponentState struct {
	Component string
	Instance  string
	Replicas  *int32
}

// WorkloadRecord is the stored state of one Karta-described workload.
// Records are immutable once inserted; callers build a fresh record per event.
type WorkloadRecord struct {
	UID        types.UID
	Ref        WorkloadRef
	Karta      string
	Generation int64
	HasStatus  bool
	Phases     []v1alpha1.ResourceStatus
	Components []ComponentState
}

// PodRecord is the stored attribution of one pod.
// Records are immutable once inserted; callers build a fresh record per event.
type PodRecord struct {
	UID         types.UID
	Namespace   string
	Name        string
	WorkloadUID types.UID
	Workload    WorkloadRef
	Component   string
	Instance    string
	Replica     string
	Reason      string
	Phase       corev1.PodPhase
}

// Snapshot is a consistent view of the store, safe to read without locks.
type Snapshot struct {
	Workloads []WorkloadRecord
	Pods      []PodRecord
}

// Store holds workload and pod records keyed by UID, with a reverse
// workload-to-pods index so workload events can re-attribute only their pods.
type Store struct {
	mu             sync.RWMutex
	workloads      map[types.UID]WorkloadRecord
	pods           map[types.UID]PodRecord
	podsByWorkload map[types.UID]map[types.UID]struct{}
}

func New() *Store {
	return &Store{
		workloads:      make(map[types.UID]WorkloadRecord),
		pods:           make(map[types.UID]PodRecord),
		podsByWorkload: make(map[types.UID]map[types.UID]struct{}),
	}
}

func (s *Store) UpsertWorkload(record WorkloadRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workloads[record.UID] = record
}

func (s *Store) DeleteWorkload(uid types.UID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.workloads, uid)
	for podUID := range s.podsByWorkload[uid] {
		delete(s.pods, podUID)
	}
	delete(s.podsByWorkload, uid)
}

func (s *Store) UpsertPod(record PodRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.pods[record.UID]; ok && existing.WorkloadUID != record.WorkloadUID {
		s.unindexPodLocked(existing)
	}
	s.pods[record.UID] = record

	pods, ok := s.podsByWorkload[record.WorkloadUID]
	if !ok {
		pods = make(map[types.UID]struct{})
		s.podsByWorkload[record.WorkloadUID] = pods
	}
	pods[record.UID] = struct{}{}
}

func (s *Store) DeletePod(uid types.UID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.pods[uid]
	if !ok {
		return
	}
	delete(s.pods, uid)
	s.unindexPodLocked(record)
}

func (s *Store) unindexPodLocked(record PodRecord) {
	pods, ok := s.podsByWorkload[record.WorkloadUID]
	if !ok {
		return
	}
	delete(pods, record.UID)
	if len(pods) == 0 {
		delete(s.podsByWorkload, record.WorkloadUID)
	}
}

// PodsOfWorkload returns copies of the pod records attributed to a workload.
func (s *Store) PodsOfWorkload(uid types.UID) []PodRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pods := make([]PodRecord, 0, len(s.podsByWorkload[uid]))
	for podUID := range s.podsByWorkload[uid] {
		pods = append(pods, s.pods[podUID])
	}
	return pods
}

// Pod returns the stored record for a pod UID.
func (s *Store) Pod(uid types.UID) (PodRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.pods[uid]
	return record, ok
}

// Workload returns the stored record for a workload UID.
func (s *Store) Workload(uid types.UID) (WorkloadRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.workloads[uid]
	return record, ok
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := Snapshot{
		Workloads: make([]WorkloadRecord, 0, len(s.workloads)),
		Pods:      make([]PodRecord, 0, len(s.pods)),
	}
	for _, record := range s.workloads {
		snapshot.Workloads = append(snapshot.Workloads, record)
	}
	for _, record := range s.pods {
		snapshot.Pods = append(snapshot.Pods, record)
	}
	return snapshot
}
