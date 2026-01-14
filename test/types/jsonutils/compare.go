package jsonutils

import (
	"encoding/json"

	"github.com/onsi/gomega"
	gtypes "github.com/onsi/gomega/types"
)

func BeJSONEquivalentTo(expected interface{}) gtypes.GomegaMatcher {
	expectedJSON, err := json.Marshal(expected)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return gomega.WithTransform(func(actual interface{}) []byte {
		b, err := json.Marshal(actual)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		return b
	}, gomega.MatchJSON(expectedJSON))
}
