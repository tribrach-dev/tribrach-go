package conformance

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/tribrach-dev/tribrach-go/annotation"
	"github.com/tribrach-dev/tribrach-go/drift"
	"github.com/tribrach-dev/tribrach-go/manifest"
	"github.com/tribrach-dev/tribrach-go/replay"
)

func load(t *testing.T, name string, v any) {
	t.Helper()
	data, err := os.ReadFile("testdata/core/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

// toJSONMap renders a value the way the conformance files write it.
func toJSONMap(t *testing.T, v any) any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAnnotations(t *testing.T) {
	var file struct {
		Cases []struct {
			Line   string `json:"line"`
			Expect any    `json:"expect"`
		} `json:"cases"`
	}
	load(t, "annotations.json", &file)
	for _, c := range file.Cases {
		d, ok := annotation.Parse(c.Line)
		var got any
		switch {
		case !ok:
			got = nil
		case d.Problem != "":
			got = map[string]any{"error": string(d.Problem)}
		case d.Kind == annotation.Link:
			got = map[string]any{"directive": "link", "lean": d.Lean}
		case d.Kind == annotation.Stamp:
			got = map[string]any{"directive": "stamp", "stamp": d.Stamp}
		default:
			got = map[string]any{"directive": "pure"}
		}
		if !reflect.DeepEqual(got, c.Expect) {
			t.Errorf("%q: got %v, want %v", c.Line, got, c.Expect)
		}
	}
}

func TestDrift(t *testing.T) {
	var file struct {
		Owner string `json:"owner"`
		Cases []struct {
			ID       string          `json:"id"`
			Manifest []manifest.Link `json:"manifest"`
			Host     []drift.Host    `json:"host"`
			Expect   any             `json:"expect"`
		} `json:"cases"`
	}
	load(t, "drift.json", &file)
	if file.Owner != manifest.Owner {
		t.Fatalf("cases are for %s", file.Owner)
	}
	for _, c := range file.Cases {
		got := toJSONMap(t, drift.Compare(file.Owner, c.Manifest, c.Host))
		if got == nil {
			got = []any{}
		}
		if !reflect.DeepEqual(got, c.Expect) {
			t.Errorf("%s: got %v, want %v", c.ID, got, c.Expect)
		}
	}
}

// fixed replays states given in advance, as the conformance file's host.
type fixed struct {
	states []json.RawMessage
	cur    json.RawMessage
	step   int
}

func (f *fixed) Start(s json.RawMessage) error { f.cur = s; return nil }
func (f *fixed) Apply(replay.Op) error         { f.cur = f.states[f.step]; f.step++; return nil }
func (f *fixed) State() (any, error)           { return f.cur, nil }

func TestReplay(t *testing.T) {
	var file struct {
		Vectors json.RawMessage              `json:"vectors"`
		Host    map[string][]json.RawMessage `json:"host"`
		Expect  any                          `json:"expect"`
	}
	load(t, "replay.json", &file)
	v, err := manifest.ParseVectors(file.Vectors)
	if err != nil {
		t.Fatal(err)
	}
	var results []replay.Result
	for _, c := range v.Cases {
		one := &manifest.Vectors{Format: v.Format, Model: v.Model, Cases: []manifest.Case{c}}
		results = append(results, replay.Check(one, func() replay.Harness { return &fixed{states: file.Host[c.ID]} })...)
	}
	if got := toJSONMap(t, results); !reflect.DeepEqual(got, file.Expect) {
		t.Errorf("got %v, want %v", got, file.Expect)
	}
}
