package resource

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRid(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ResourceInterface Suite")
}
