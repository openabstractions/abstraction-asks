// Package pendingquestioncases supplies the shared INF-W11 / ASK-A14
// refusal-word corpus for Go consumer compatibility tests.
package pendingquestioncases

import (
	_ "embed"
	"encoding/json"
)

// Case includes the existing consumer classifiers, whose grant words differ.
type Case struct {
	Name, Reason string
	IDs          []string
	Job, Service bool
}

//go:embed cases.json
var data []byte

// Cases returns an independent copy of the shared compatibility corpus.
func Cases() []Case {
	var cases []Case
	if err := json.Unmarshal(data, &cases); err != nil {
		panic(err)
	}
	return cases
}
