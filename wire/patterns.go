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
	// Calendar: the YYYY-MM-DD the string starts with must be a day that
	// exists, which no tree or regex says (core's Pat.calendar).
	Calendar bool
}

// Check reports whether s matches the pattern.
func (p Pattern) Check(s string) bool { return p.Tree.Match(s) && (!p.Calendar || RealDate(s)) }

// RealDate reports whether the YYYY-MM-DD at the start of s is a day that
// exists in the proleptic Gregorian calendar.
func RealDate(s string) bool {
	if len(s) < 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	num := func(a, b int) int {
		n := 0
		for _, c := range s[a:b] {
			if c < '0' || c > '9' {
				return -1
			}
			n = n*10 + int(c-'0')
		}
		return n
	}
	y, m, d := num(0, 4), num(5, 7), num(8, 10)
	if y < 0 || m < 1 || m > 12 || d < 1 {
		return false
	}
	days := [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}[m-1]
	if m == 2 && ((y%4 == 0 && y%100 != 0) || y%400 == 0) {
		days = 29
	}
	return d <= days
}

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
			Name     string `json:"name"`
			Regex    string `json:"regex"`
			Tree     Tree   `json:"tree"`
			Calendar bool   `json:"calendar"`
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
		ps[i] = Pattern{c.Name, c.Regex, c.Tree, c.Calendar}
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
