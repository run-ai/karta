// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package owner

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOwner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Owner Suite")
}
