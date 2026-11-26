package wire

import (
	"strings"
	"unicode/utf8"
)

// A Pattern is one of core's closed list of string patterns
// (Tribrach.Schema.Pat). Schemas carry each one's regex; generated code
// finds the checker by that regex, so no regex is written or run here.
type Pattern struct {
	Name  string
	Regex string
	Check func(string) bool
}

// Patterns are the patterns this version knows, as listed in core's
// tests/conformance/patterns.json.
var Patterns = []Pattern{
	{"hex16", "^[0-9a-f]{16}$", hexN(16)},
	{"hex40", "^[0-9a-f]{40}$", hexN(40)},
	{"hex64", "^[0-9a-f]{64}$", hexN(64)},
	{"slug", "^[a-z0-9][a-z0-9-]*$", slug},
	{"upperAlnum", "^[A-Z0-9]+$", upperAlnum},
	{"ownerRepo", "^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$", ownerRepo},
	{"relPath", `^(?!/)(?!.*(^|/)\.\.(/|$))[^\\\x00-\x1f\u2028\u2029]+$`, relPath},
	{"uri", `^[A-Za-z][A-Za-z0-9+.-]*:[^\x00-\x20\x7f]*$`, uri},
	{"dateTime", `^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$`, dateTime},
	{"instant", `^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`, instant},
	{"decimal", "^(0|-?[1-9][0-9]*)$", decimal},
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

func isDigit(c rune) bool { return '0' <= c && c <= '9' }
func isLower(c rune) bool { return 'a' <= c && c <= 'z' }
func isUpper(c rune) bool { return 'A' <= c && c <= 'Z' }

func all(s string, f func(rune) bool) bool {
	for _, c := range s {
		if !f(c) {
			return false
		}
	}
	return true
}

func hexN(n int) func(string) bool {
	return func(s string) bool {
		return utf8.RuneCountInString(s) == n && all(s, func(c rune) bool { return isDigit(c) || ('a' <= c && c <= 'f') })
	}
}

func slug(s string) bool {
	if s == "" {
		return false
	}
	c, _ := utf8.DecodeRuneInString(s)
	return (isLower(c) || isDigit(c)) && all(s, func(c rune) bool { return isLower(c) || isDigit(c) || c == '-' })
}

func upperAlnum(s string) bool {
	return s != "" && all(s, func(c rune) bool { return isUpper(c) || isDigit(c) })
}

func ownerRepo(s string) bool {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return false
	}
	ok := func(c rune) bool { return isLower(c) || isUpper(c) || isDigit(c) || c == '_' || c == '.' || c == '-' }
	return parts[0] != "" && parts[1] != "" && all(parts[0], ok) && all(parts[1], ok)
}

func relPath(s string) bool {
	if s == "" || strings.HasPrefix(s, "/") {
		return false
	}
	if !all(s, func(c rune) bool { return c != '\\' && c >= 0x20 && c != '\u2028' && c != '\u2029' }) {
		return false
	}
	for _, seg := range strings.Split(s, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

func uri(s string) bool {
	i := strings.IndexByte(s, ':')
	if i < 1 {
		return false
	}
	scheme, rest := s[:i], s[i+1:]
	c, _ := utf8.DecodeRuneInString(scheme)
	if !(isLower(c) || isUpper(c)) {
		return false
	}
	if !all(scheme, func(c rune) bool { return isLower(c) || isUpper(c) || isDigit(c) || c == '+' || c == '.' || c == '-' }) {
		return false
	}
	return all(rest, func(c rune) bool { return c > 0x20 && c != 0x7f })
}

// scanner reads a pattern left to right over the string's runes.
type scanner struct {
	rs []rune
	ok bool
}

func (s *scanner) digits(n int) []int {
	out := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if !s.ok || len(s.rs) == 0 || !isDigit(s.rs[0]) {
			s.ok = false
			return nil
		}
		out = append(out, int(s.rs[0]-'0'))
		s.rs = s.rs[1:]
	}
	return out
}

func (s *scanner) num(n int) int {
	v := 0
	for _, d := range s.digits(n) {
		v = v*10 + d
	}
	return v
}

func (s *scanner) lit(c rune) {
	if !s.ok || len(s.rs) == 0 || s.rs[0] != c {
		s.ok = false
		return
	}
	s.rs = s.rs[1:]
}

func (s *scanner) offset(maxHour, maxMin int) bool {
	if !s.ok || len(s.rs) == 0 {
		return false
	}
	if len(s.rs) == 1 && s.rs[0] == 'Z' {
		return true
	}
	if s.rs[0] != '+' && s.rs[0] != '-' {
		return false
	}
	s.rs = s.rs[1:]
	h := s.num(2)
	s.lit(':')
	m := s.num(2)
	return s.ok && len(s.rs) == 0 && h <= maxHour && m <= maxMin
}

func dateTime(str string) bool {
	s := &scanner{[]rune(str), true}
	s.digits(4)
	s.lit('-')
	s.digits(2)
	s.lit('-')
	s.digits(2)
	s.lit('T')
	s.digits(2)
	s.lit(':')
	s.digits(2)
	s.lit(':')
	s.digits(2)
	if s.ok && len(s.rs) > 0 && s.rs[0] == '.' {
		s.rs = s.rs[1:]
		if len(s.rs) == 0 || !isDigit(s.rs[0]) {
			return false
		}
		for len(s.rs) > 0 && isDigit(s.rs[0]) {
			s.rs = s.rs[1:]
		}
	}
	return s.offset(99, 99)
}

func instant(str string) bool {
	s := &scanner{[]rune(str), true}
	s.digits(4)
	s.lit('-')
	mo := s.num(2)
	s.lit('-')
	d := s.num(2)
	s.lit('T')
	h := s.num(2)
	s.lit(':')
	mi := s.num(2)
	s.lit(':')
	se := s.num(2)
	if !s.ok || mo < 1 || mo > 12 || d < 1 || d > 31 || h > 23 || mi > 59 || se > 59 {
		return false
	}
	return s.offset(23, 59)
}

func decimal(s string) bool {
	if s == "0" {
		return true
	}
	s = strings.TrimPrefix(s, "-")
	return s != "" && s[0] != '0' && all(s, isDigit)
}
