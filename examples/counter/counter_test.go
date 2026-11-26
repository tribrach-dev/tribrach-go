package counter

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/tribrach-dev/tribrach-go/manifest"
	"github.com/tribrach-dev/tribrach-go/replay"
)

// harness drives a step function through the vectors.
type harness struct {
	max  uint
	n    uint
	step func(max uint, op Op, n uint) uint
}

func (h *harness) Start(s json.RawMessage) error { return json.Unmarshal(s, &h.n) }
func (h *harness) State() (any, error)           { return h.n, nil }
func (h *harness) Apply(op replay.Op) error {
	switch Op(op.Name) {
	case Incr, Reset:
		h.n = h.step(h.max, Op(op.Name), h.n)
		return nil
	}
	return fmt.Errorf("unknown operation %q", op.Name)
}

func TestStepMatchesModel(t *testing.T) {
	replay.Run(t, "testdata/counter.vectors.json", func() replay.Harness {
		return &harness{max: 3, step: Step}
	})
}

// The vectors must catch a plausible bug: allowing the counter to reach
// max + 1.
func TestVectorsCatchOffByOne(t *testing.T) {
	buggy := func(max uint, op Op, n uint) uint {
		if op == Incr && n <= max {
			return n + 1
		}
		return Step(max, op, n)
	}
	v, err := manifest.LoadVectors("testdata/counter.vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	failed := 0
	for _, r := range replay.Check(v, func() replay.Harness { return &harness{max: 3, step: buggy} }) {
		if r.Outcome != "ok" {
			failed++
			t.Log(r)
		}
	}
	if failed == 0 {
		t.Error("the vectors don't catch the off-by-one")
	}
}
