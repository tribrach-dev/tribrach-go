// Package annotation reads //tribrach: directives from comment lines.
//
// The grammar is shared by every Tribrach instrument and pinned by the core
// conformance cases (tests/conformance/annotations.json):
//
//	//tribrach:link <LeanName>
//	//tribrach:pure
//	//tribrach:stamp <16 lowercase hex digits>
//
// A directive has no space after the //, like //go: directives; with a space
// it is an ordinary comment.
package annotation

import (
	"regexp"
	"strings"
)

// Kind names a directive.
type Kind string

const (
	Link  Kind = "link"
	Pure  Kind = "pure"
	Stamp Kind = "stamp"
)

// Problem names why a directive couldn't be read. The values are the
// conformance outcome names.
type Problem string

const (
	MissingArgument  Problem = "missing-argument"
	ExtraArgument    Problem = "extra-argument"
	BadStamp         Problem = "bad-stamp"
	UnknownDirective Problem = "unknown-directive"
)

// Directive is one read directive. Exactly one of Kind and Problem is set.
type Directive struct {
	Kind    Kind
	Lean    string // for Link
	Stamp   string // for Stamp
	Problem Problem
}

const prefix = "tribrach:"

var stampRE = regexp.MustCompile(`^[0-9a-f]{16}$`)

// IsStamp reports whether s is a well-formed stamp.
func IsStamp(s string) bool { return stampRE.MatchString(s) }

// splitArgs splits on runs of spaces and tabs, keeping «...» groups whole.
func splitArgs(s string) []string {
	var args []string
	var cur strings.Builder
	quoted := false
	for _, r := range s {
		switch {
		case r == '«':
			quoted = true
		case r == '»':
			quoted = false
		}
		if (r == ' ' || r == '\t') && !quoted {
			if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args
}

// Parse reads one comment line, including its // marker. ok is false when
// the line isn't a directive at all.
func Parse(line string) (d Directive, ok bool) {
	body, found := strings.CutPrefix(line, "//")
	if !found {
		return Directive{}, false
	}
	rest, found := strings.CutPrefix(body, prefix)
	if !found {
		return Directive{}, false
	}
	rest = strings.TrimRight(rest, " \t")
	if rest == "" || rest[0] == ' ' || rest[0] == '\t' {
		return Directive{Problem: UnknownDirective}, true
	}
	words := splitArgs(rest)
	name, args := words[0], words[1:]
	arity := map[string]int{"link": 1, "pure": 0, "stamp": 1}
	n, known := arity[name]
	switch {
	case !known:
		return Directive{Problem: UnknownDirective}, true
	case len(args) < n:
		return Directive{Problem: MissingArgument}, true
	case len(args) > n:
		return Directive{Problem: ExtraArgument}, true
	}
	switch Kind(name) {
	case Link:
		return Directive{Kind: Link, Lean: args[0]}, true
	case Stamp:
		if !IsStamp(args[0]) {
			return Directive{Problem: BadStamp}, true
		}
		return Directive{Kind: Stamp, Stamp: args[0]}, true
	default:
		return Directive{Kind: Pure}, true
	}
}
