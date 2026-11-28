package wire

import (
	"encoding/json"
	"fmt"
	"strings"
)

// A Tree is a pattern tree from core's tests/conformance/patterns.json
// (Tribrach.Re). Exactly one of the node kinds is set:
//
//	{"lit": s}                        s itself
//	{"set": items} / {"notSet": items} one character in (or not in) the set
//	{"seq": [..]}                     each in turn
//	{"alt": [..]}                     any one
//	{"rep": t, "min": m, "max": n}    t from m to n times (null: no limit)
//	{"without": t, "bans": [..]}      t, refusing what a ban matches
type Tree struct {
	Lit     *string `json:"lit,omitempty"`
	Set     []Item  `json:"set,omitempty"`
	NotSet  []Item  `json:"notSet,omitempty"`
	Seq     []Tree  `json:"seq,omitempty"`
	Alt     []Tree  `json:"alt,omitempty"`
	Rep     *Tree   `json:"rep,omitempty"`
	Min     int     `json:"min,omitempty"`
	Max     *int    `json:"max,omitempty"`
	Without *Tree   `json:"without,omitempty"`
	Bans    []Ban   `json:"bans,omitempty"`
}

// An Item is one member of a set: a character, or an inclusive range.
type Item struct{ Lo, Hi rune }

// UnmarshalJSON reads "c" or ["lo", "hi"].
func (it *Item) UnmarshalJSON(data []byte) error {
	var one string
	if json.Unmarshal(data, &one) == nil {
		r := []rune(one)
		if len(r) != 1 {
			return fmt.Errorf("set item %q is not one character", one)
		}
		*it = Item{r[0], r[0]}
		return nil
	}
	var pair [2]string
	if err := json.Unmarshal(data, &pair); err != nil {
		return err
	}
	lo, hi := []rune(pair[0]), []rune(pair[1])
	if len(lo) != 1 || len(hi) != 1 {
		return fmt.Errorf("set range %q is not two characters", pair)
	}
	*it = Item{lo[0], hi[0]}
	return nil
}

// A Ban is what a without node refuses, judged on the rest of the string:
// it starts with StartsWith, or splitting on Sep gives a part equal to
// Segment.
type Ban struct {
	StartsWith *string `json:"startsWith,omitempty"`
	Segment    *string `json:"segment,omitempty"`
	Sep        string  `json:"sep,omitempty"`
}

func (b Ban) hits(rest string) bool {
	if b.StartsWith != nil {
		return strings.HasPrefix(rest, *b.StartsWith)
	}
	if b.Segment != nil {
		for _, part := range strings.Split(rest, b.Sep) {
			if part == *b.Segment {
				return true
			}
		}
	}
	return false
}

func inSet(items []Item, c rune) bool {
	for _, it := range items {
		if it.Lo <= c && c <= it.Hi {
			return true
		}
	}
	return false
}

type positions map[int]bool

// steps gives every position t can stop at, starting from s[i].
func (t *Tree) steps(s []rune, i int) positions {
	out := positions{}
	switch {
	case t.Lit != nil:
		lit := []rune(*t.Lit)
		if i+len(lit) <= len(s) && string(s[i:i+len(lit)]) == *t.Lit {
			out[i+len(lit)] = true
		}
	case t.Set != nil || t.NotSet != nil:
		neg := t.NotSet != nil
		items := t.Set
		if neg {
			items = t.NotSet
		}
		if i < len(s) && inSet(items, s[i]) != neg {
			out[i+1] = true
		}
	case t.Seq != nil:
		front := positions{i: true}
		for k := range t.Seq {
			next := positions{}
			for j := range front {
				for e := range t.Seq[k].steps(s, j) {
					next[e] = true
				}
			}
			front = next
		}
		return front
	case t.Alt != nil:
		for k := range t.Alt {
			for e := range t.Alt[k].steps(s, i) {
				out[e] = true
			}
		}
	case t.Rep != nil:
		front := positions{i: true}
		for n := 0; len(front) > 0; n++ {
			if n >= t.Min {
				for j := range front {
					out[j] = true
				}
			}
			if t.Max != nil && n == *t.Max {
				break
			}
			next := positions{}
			for j := range front {
				for e := range t.Rep.steps(s, j) {
					// A round that consumes nothing adds nothing once min is met.
					if e > j || n < t.Min {
						next[e] = true
					}
				}
			}
			front = next
		}
	case t.Without != nil:
		rest := string(s[i:])
		for _, b := range t.Bans {
			if b.hits(rest) {
				return out
			}
		}
		return t.Without.steps(s, i)
	}
	return out
}

// Match reports whether the tree matches all of s.
func (t *Tree) Match(s string) bool {
	r := []rune(s)
	return t.steps(r, 0)[len(r)]
}
