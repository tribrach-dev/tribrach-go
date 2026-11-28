package wire

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestEmbeddedPatterns checks that the patterns compiled in are core's.
func TestEmbeddedPatterns(t *testing.T) {
	data, err := os.ReadFile("../conformance/testdata/core/patterns.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, patternsJSON) {
		t.Fatal("TRG0403 wire/patterns.json differs from core's tests/conformance/patterns.json; fix: copy conformance/testdata/core/patterns.json to wire/patterns.json")
	}
}

// TestPatternsConformance checks every pattern against core's cases
// (conformance/testdata/core/patterns.json): the regex must be core's,
// byte for byte, and both the tree matcher and the regex must accept and
// reject exactly what core says.
func TestPatternsConformance(t *testing.T) {
	data, err := os.ReadFile("../conformance/testdata/core/patterns.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Format string `json:"format"`
		Cases  []struct {
			Name     string   `json:"name"`
			Regex    string   `json:"regex"`
			Calendar bool     `json:"calendar"`
			Accept   []string `json:"accept"`
			Reject   []string `json:"reject"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if file.Format != "tribrach.conformance.patterns.v1" {
		t.Fatalf("format %q", file.Format)
	}
	seen := map[string]bool{}
	for _, c := range file.Cases {
		seen[c.Name] = true
		p, ok := PatternByName(c.Name)
		if !ok {
			t.Errorf("TRG0403 core has pattern %s, which wire doesn't know; fix: copy core's patterns.json to wire/patterns.json", c.Name)
			continue
		}
		if p.Regex != c.Regex {
			t.Errorf("TRG0403 %s: regex %q, core has %q", c.Name, p.Regex, c.Regex)
		}
		if q, ok := PatternByRegex(c.Regex); !ok || q.Name != c.Name {
			t.Errorf("TRG0403 %s: PatternByRegex doesn't find it", c.Name)
		}
		// Go's regexp has no lookahead, so a regex using it (relPath) is
		// checked by its tree alone. Elsewhere, \z is ECMA-262's final $.
		rx, err := regexp.Compile(strings.TrimSuffix(c.Regex, "$") + `\z`)
		if err != nil {
			rx = nil
		}
		// A calendar pattern's regex checks the shape; the day is checked apart.
		var re interface{ MatchString(string) bool }
		if rx != nil {
			re = calendarRegex{rx, c.Calendar}
		}
		for _, s := range c.Accept {
			if !p.Check(s) {
				t.Errorf("%s: the tree rejects %q, core accepts it", c.Name, s)
			}
			if re != nil && !re.MatchString(s) {
				t.Errorf("%s: regex rejects %q, core accepts it", c.Name, s)
			}
		}
		for _, s := range c.Reject {
			if p.Check(s) {
				t.Errorf("%s: the tree accepts %q, core rejects it", c.Name, s)
			}
			if re != nil && re.MatchString(s) {
				t.Errorf("%s: regex accepts %q, core rejects it", c.Name, s)
			}
		}
	}
	for _, p := range Patterns {
		if !seen[p.Name] {
			t.Errorf("TRG0403 wire has pattern %s, which core doesn't; core defines patterns", p.Name)
		}
	}
}

type calendarRegex struct {
	re       *regexp.Regexp
	calendar bool
}

func (c calendarRegex) MatchString(s string) bool {
	return c.re.MatchString(s) && (!c.calendar || RealDate(s))
}
