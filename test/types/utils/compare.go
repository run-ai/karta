package utils

import (
	"encoding/json"

	gomega "github.com/onsi/gomega"
	gtypes "github.com/onsi/gomega/types"
)

func BeJSONEquivalentTo(expected interface{}) gtypes.GomegaMatcher {
	expectedJSON, _ := json.Marshal(expected)

	return gomega.WithTransform(func(actual interface{}) []byte {
		b, _ := json.Marshal(actual)
		return b
	}, gomega.MatchJSON(expectedJSON))
}
