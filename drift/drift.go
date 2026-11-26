// Package drift compares the links found in Go code with the manifest. The
// outcomes are the core conformance names (tests/conformance/drift.json).
package drift

import (
	"sort"

	"github.com/tribrach-dev/tribrach-go/manifest"
)

type Outcome string

const (
	OK             Outcome = "ok"
	Drift          Outcome = "drift"
	Unstamped      Outcome = "unstamped"
	Unknown        Outcome = "unknown"
	SymbolMismatch Outcome = "symbol-mismatch"
	Missing        Outcome = "missing"
)

// Host is one annotated host symbol. Stamp is empty when none is recorded.
type Host struct {
	Symbol string `json:"symbol"`
	Lean   string `json:"lean"`
	Stamp  string `json:"stamp,omitempty"`
}

// Finding is one comparison result. Want is the manifest's stamp for drift
// and unstamped, and the manifest's symbol for symbol-mismatch.
type Finding struct {
	Lean    string  `json:"lean"`
	Symbol  string  `json:"symbol"`
	Outcome Outcome `json:"outcome"`
	Want    string  `json:"want,omitempty"`
}

// Compare checks host links against the manifest links owned by owner,
// sorted by Lean name then symbol.
func Compare(owner string, links []manifest.Link, host []Host) []Finding {
	type key struct{ lean, symbol string }
	mine := map[key]manifest.Link{}
	symbols := map[string][]string{}
	for _, l := range links {
		if l.Owner != owner {
			continue
		}
		mine[key{l.Lean, l.Symbol}] = l
		symbols[l.Lean] = append(symbols[l.Lean], l.Symbol)
	}
	var out []Finding
	seen := map[key]bool{}
	for _, h := range host {
		k := key{h.Lean, h.Symbol}
		if l, ok := mine[k]; ok {
			seen[k] = true
			switch {
			case h.Stamp == "":
				out = append(out, Finding{h.Lean, h.Symbol, Unstamped, l.Stamp})
			case h.Stamp != l.Stamp:
				out = append(out, Finding{h.Lean, h.Symbol, Drift, l.Stamp})
			default:
				out = append(out, Finding{h.Lean, h.Symbol, OK, ""})
			}
		} else if syms := symbols[h.Lean]; len(syms) > 0 {
			s := append([]string(nil), syms...)
			sort.Strings(s)
			out = append(out, Finding{h.Lean, h.Symbol, SymbolMismatch, s[0]})
		} else {
			out = append(out, Finding{h.Lean, h.Symbol, Unknown, ""})
		}
	}
	for k, l := range mine {
		if !seen[k] {
			out = append(out, Finding{l.Lean, l.Symbol, Missing, ""})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Lean != out[j].Lean {
			return out[i].Lean < out[j].Lean
		}
		return out[i].Symbol < out[j].Symbol
	})
	return out
}
