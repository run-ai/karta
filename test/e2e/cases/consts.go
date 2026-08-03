// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

// State aliases, exported so flow tests can name them without the kartav1alpha1 qualifier.
const (
	Initializing = kartav1alpha1.InitializingStatus
	Running      = kartav1alpha1.RunningStatus
	Completed    = kartav1alpha1.CompletedStatus
	Failed       = kartav1alpha1.FailedStatus
	Suspended    = kartav1alpha1.SuspendedStatus
	Degraded     = kartav1alpha1.DegradedStatus
)
