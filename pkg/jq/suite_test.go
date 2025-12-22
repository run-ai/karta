package jq

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestJq(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Jq Suite")
}
