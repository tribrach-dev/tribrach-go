package wire

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// A Pattern is one of core's closed list of string patterns
// (Tribrach.Schema.Pat). Schemas carry each one's regex; generated code
// finds the pattern by that regex and checks strings with its tree, so no
// regex is written or run here.
type Pattern struct {
	Name  string
	Regex string
	Tree  Tree
}

// Check reports whether s matches the pattern.
func (p Pattern) Check(s string) bool { return p.Tree.Match(s) }

// patternsJSON is core's tests/conformance/patterns.json, copied verbatim;
// a test checks it against conformance/testdata/core.
//
//go:embed patterns.json
var patternsJSON []byte

// Patterns are the patterns this version knows, read from core's
// tests/conformance/patterns.json.
var Patterns = loadPatterns(patternsJSON)

func loadPatterns(data []byte) []Pattern {
	var file struct {
		Format string `json:"format"`
		Cases  []struct {
			Name  string `json:"name"`
			Regex string `json:"regex"`
			Tree  Tree   `json:"tree"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		panic(fmt.Sprintf("wire: the embedded patterns.json doesn't read: %v", err))
	}
	if file.Format != "tribrach.conformance.patterns.v1" {
		panic(fmt.Sprintf("wire: the embedded patterns.json has format %q", file.Format))
	}
	ps := make([]Pattern, len(file.Cases))
	for i, c := range file.Cases {
		ps[i] = Pattern{c.Name, c.Regex, c.Tree}
	}
	return ps
}

// PatternByRegex finds a known pattern by its exported regex.
func PatternByRegex(regex string) (Pattern, bool) {
	for _, p := range Patterns {
		if p.Regex == regex {
			return p, true
		}
	}
	return Pattern{}, false
}

// PatternByName finds a known pattern by name.
func PatternByName(name string) (Pattern, bool) {
	for _, p := range Patterns {
		if p.Name == name {
			return p, true
		}
	}
	return Pattern{}, false
}
