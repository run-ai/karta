// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package workload

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWorkload(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Workload Suite")
}
