package jsonutils

import (
	"encoding/json"

	. "github.com/onsi/gomega"
	gomega "github.com/onsi/gomega"
	gtypes "github.com/onsi/gomega/types"
)

func BeJSONEquivalentTo(expected interface{}) gtypes.GomegaMatcher {
	expectedJSON, err := json.Marshal(expected)
	Expect(err).NotTo(HaveOccurred())

	return gomega.WithTransform(func(actual interface{}) []byte {
		b, err := json.Marshal(actual)
		Expect(err).NotTo(HaveOccurred())

		return b
	}, gomega.MatchJSON(expectedJSON))
}
