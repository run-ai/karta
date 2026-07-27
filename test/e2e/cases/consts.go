// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

// State names, from the Karta ResourceStatus enum. A case's registry maps a workload's own fields
// to these; a journey step names one. Kept unexported: the case files reuse the same lowercase words
// ("running", "completed", ...) as plain-string flow names, which must stay strings.
const (
	initializing = kartav1alpha1.InitializingStatus
	running      = kartav1alpha1.RunningStatus
	completed    = kartav1alpha1.CompletedStatus
	failed       = kartav1alpha1.FailedStatus
	suspended    = kartav1alpha1.SuspendedStatus
	degraded     = kartav1alpha1.DegradedStatus
)
