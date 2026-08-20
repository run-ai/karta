// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package definitions

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDefinitions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Definitions Suite")
}
