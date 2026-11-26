// Package replay runs exported Tribrach vectors against a Go implementation.
//
// A test supplies a Harness that starts from a state, applies operations and
// reports its state as JSON in the model's encoding:
//
//	func TestCounterMatchesModel(t *testing.T) {
//		replay.Run(t, "testdata/counter.vectors.json", func() replay.Harness { return &counterHarness{max: 3} })
//	}
//
// Each case runs as a subtest. A case stops at the first step whose state
// differs from the model's, and the failure names the step, the operation
// and both states (tests/conformance/replay.json in the core repository).
package replay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/tribrach-dev/tribrach-go/manifest"
)

// Op is one operation: its name (the "op" field) and the whole object.
type Op struct {
	Name string
	Raw  json.RawMessage
}

// Decode unmarshals the operation's fields into v.
func (o Op) Decode(v any) error { return json.Unmarshal(o.Raw, v) }

// Harness drives the implementation under test.
type Harness interface {
	// Start resets the implementation to the given state.
	Start(state json.RawMessage) error
	// Apply performs one operation.
	Apply(op Op) error
	// State returns the current state; it is encoded with encoding/json.
	State() (any, error)
}

// Result is the outcome of one case: "ok", "differs" or "error".
type Result struct {
	Case    string          `json:"case"`
	Outcome string          `json:"outcome"`
	Step    int             `json:"step,omitempty"`
	Op      json.RawMessage `json:"op,omitempty"`
	Want    json.RawMessage `json:"want,omitempty"`
	Got     json.RawMessage `json:"got,omitempty"`
	Error   string          `json:"error,omitempty"`
}

func (r Result) String() string {
	switch r.Outcome {
	case "differs":
		return fmt.Sprintf("case %s, step %d (%s): the model has %s, the code has %s", r.Case, r.Step, r.Op, r.Want, r.Got)
	case "error":
		return fmt.Sprintf("case %s, step %d: %s", r.Case, r.Step, r.Error)
	}
	return "case " + r.Case + ": ok"
}

// Check runs every case with a fresh harness.
func Check(v *manifest.Vectors, newHarness func() Harness) []Result {
	var out []Result
	for _, c := range v.Cases {
		out = append(out, checkCase(c, newHarness()))
	}
	return out
}

func checkCase(c manifest.Case, h Harness) Result {
	if err := h.Start(c.Start); err != nil {
		return Result{Case: c.ID, Outcome: "error", Error: "start: " + err.Error()}
	}
	for i, raw := range c.Ops {
		var head struct {
			Op string `json:"op"`
		}
		if err := json.Unmarshal(raw, &head); err != nil || head.Op == "" {
			return Result{Case: c.ID, Outcome: "error", Step: i + 1, Error: "operation has no op field"}
		}
		if err := h.Apply(Op{head.Op, raw}); err != nil {
			return Result{Case: c.ID, Outcome: "error", Step: i + 1, Op: raw, Error: err.Error()}
		}
		st, err := h.State()
		if err != nil {
			return Result{Case: c.ID, Outcome: "error", Step: i + 1, Op: raw, Error: "state: " + err.Error()}
		}
		got, err := json.Marshal(st)
		if err != nil {
			return Result{Case: c.ID, Outcome: "error", Step: i + 1, Op: raw, Error: "state: " + err.Error()}
		}
		if !Equal(c.States[i], got) {
			return Result{Case: c.ID, Outcome: "differs", Step: i + 1, Op: raw, Want: c.States[i], Got: got}
		}
	}
	return Result{Case: c.ID, Outcome: "ok"}
}

// Run loads vectors and checks each case as a subtest.
func Run(t *testing.T, path string, newHarness func() Harness) {
	t.Helper()
	v, err := manifest.LoadVectors(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range v.Cases {
		t.Run(c.ID, func(t *testing.T) {
			if r := checkCase(c, newHarness()); r.Outcome != "ok" {
				t.Error(r.String())
			}
		})
	}
}

// Equal compares two JSON values: object key order is ignored, numbers
// compare by value (1 equals 1.0), and booleans are never numbers.
func Equal(a, b json.RawMessage) bool {
	x, err1 := decode(a)
	y, err2 := decode(b)
	return err1 == nil && err2 == nil && same(x, y)
}

func decode(raw json.RawMessage) (any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	err := d.Decode(&v)
	return v, err
}

func same(a, b any) bool {
	switch x := a.(type) {
	case json.Number:
		y, ok := b.(json.Number)
		if !ok {
			return false
		}
		rx, ok1 := new(big.Rat).SetString(string(x))
		ry, ok2 := new(big.Rat).SetString(string(y))
		return ok1 && ok2 && rx.Cmp(ry) == 0
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, v := range x {
			w, ok := y[k]
			if !ok || !same(v, w) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !same(x[i], y[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}
