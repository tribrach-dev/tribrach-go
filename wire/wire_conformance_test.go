package wire

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// TestWireConformance runs core's wire cases
// (conformance/testdata/core/wire.json): parse the input text, read it as
// the schema says with Reader, write it back, and compare with core's
// output, or check that reading fails.
func TestWireConformance(t *testing.T) {
	data, err := os.ReadFile("../conformance/testdata/core/wire.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Format string `json:"format"`
		Cases  []struct {
			ID     string         `json:"id"`
			Schema map[string]any `json:"schema"`
			Input  string         `json:"input"`
			Output any            `json:"output"`
			Error  bool           `json:"error"`
		} `json:"cases"`
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber() // outputs reach int64 limits
	if err := d.Decode(&file); err != nil {
		t.Fatal(err)
	}
	if file.Format != "tribrach.conformance.wire.v1" {
		t.Fatalf("format %q", file.Format)
	}
	for _, c := range file.Cases {
		got, err := readWrite(c.Schema, c.Input)
		switch {
		case c.Error && err == nil:
			t.Errorf("%s: reads %q as %v, core rejects it", c.ID, c.Input, got)
		case !c.Error && err != nil:
			t.Errorf("%s: rejects %q (%v), core reads it", c.ID, c.Input, err)
		case !c.Error && !sameJSON(got, c.Output):
			t.Errorf("%s: writes %v back, core writes %v", c.ID, got, c.Output)
		}
	}
}

func readWrite(schema map[string]any, input string) (any, error) {
	v, err := Parse([]byte(input))
	if err != nil {
		return nil, err
	}
	r := &Reader{Schema: "wire.json"}
	out := read(r, schema, v, "")
	if r.Err != nil {
		return nil, r.Err
	}
	return out, nil
}

func bound(m map[string]any, k string) *int64 {
	n, ok := m[k].(json.Number)
	if !ok {
		return nil
	}
	i, err := n.Int64()
	if err != nil {
		return nil
	}
	return Ptr(i)
}

// read follows the schema with Reader's primitives, as generated code
// does, and returns what generated code would write.
func read(r *Reader, s map[string]any, v any, path string) any {
	if c, ok := s["const"].(string); ok {
		return r.Enum(v, path, c)
	}
	if vals, ok := s["enum"].([]any); ok {
		names := make([]string, len(vals))
		for i, x := range vals {
			names[i] = x.(string)
		}
		return r.Enum(v, path, names...)
	}
	if alts, ok := s["oneOf"].([]any); ok {
		// Tagged union: each alternative is an object with one constant field.
		var tag string
		var names []string
		byName := map[string]map[string]any{}
		for _, a := range alts {
			alt := a.(map[string]any)
			for k, p := range alt["properties"].(map[string]any) {
				if c, ok := p.(map[string]any)["const"].(string); ok {
					tag = k
					names = append(names, c)
					byName[c] = alt
				}
			}
		}
		name := r.Tag(v, path, tag, names...)
		if r.Err != nil {
			return nil
		}
		return read(r, byName[name], v, path)
	}
	switch s["type"] {
	case "boolean":
		return r.Bool(v, path)
	case "integer":
		m := bound(s, "multipleOf")
		b := IntBounds{Min: bound(s, "minimum"), Max: bound(s, "maximum")}
		if m != nil {
			b.MultipleOf = *m
		}
		return r.Int(v, path, b)
	case "string":
		var pat Pattern
		if re, ok := s["pattern"].(string); ok {
			if pat, ok = PatternByRegex(re); !ok {
				r.fail(path, "unknown pattern %q", re)
				return nil
			}
		}
		switch pat.Name {
		case "decimal":
			x, _ := s["x-tribrach"].(map[string]any)
			return FormatDecimal(r.Decimal(v, path, IntBounds{Min: bound(x, "minimum"), Max: bound(x, "maximum")}))
		case "instant":
			return FormatInstant(r.Instant(v, path))
		}
		return r.String(v, path, StrBounds{MinLen: bound(s, "minLength"), MaxLen: bound(s, "maxLength"), Pattern: pat.Name})
	case "array":
		xs, ok := v.([]any)
		n := int64(len(xs))
		switch {
		case !ok:
			r.fail(path, "want an array")
		case bound(s, "minItems") != nil && n < *bound(s, "minItems"),
			bound(s, "maxItems") != nil && n > *bound(s, "maxItems"):
			r.fail(path, "wrong number of items")
		}
		out := []any{}
		for i, x := range xs {
			out = append(out, read(r, s["items"].(map[string]any), x, fmt.Sprintf("%s[%d]", path, i)))
		}
		return out
	case "object":
		props := s["properties"].(map[string]any)
		keys := []string{}
		for k := range props {
			keys = append(keys, k)
		}
		required := map[string]bool{}
		if rs, ok := s["required"].([]any); ok {
			for _, k := range rs {
				required[k.(string)] = true
			}
		}
		m := r.Object(v, path, keys...)
		out := map[string]any{}
		for _, k := range keys {
			p := props[k].(map[string]any)
			if required[k] {
				out[k] = read(r, p, r.Field(m, path, k), Join(path, k))
			} else if x, ok := Optional(m, k); ok {
				out[k] = read(r, p, x, Join(path, k))
			}
		}
		return out
	}
	r.fail(path, "schema not covered: %v", s)
	return nil
}

// sameJSON compares by JSON value: what read gives, marshalled, against
// core's output.
func sameJSON(got, want any) bool {
	g, err := json.Marshal(got)
	if err != nil {
		return false
	}
	var a, b any
	d := json.NewDecoder(bytes.NewReader(g))
	d.UseNumber()
	if d.Decode(&a) != nil {
		return false
	}
	w, _ := json.Marshal(want)
	d = json.NewDecoder(bytes.NewReader(w))
	d.UseNumber()
	if d.Decode(&b) != nil {
		return false
	}
	return reflect.DeepEqual(a, b)
}
