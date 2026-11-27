// Package wire is the runtime for DTOs that `tribrach-go gen` generates from
// core's JSON Schemas. It reads JSON the way core's Lean decoders do
// (Tribrach.Wire): objects are closed, null is not absent, whole numbers may
// be written 2.0, money and 64-bit ids travel as decimal strings, and
// instants are RFC 3339 read at any offset and written in UTC.
//
// Generated decoders accept exactly what the schema accepts; the parking
// example checks this against the verdicts core exports from Lean.
package wire

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Error says where a document breaks its schema.
type Error struct {
	Schema string // the schema's $id
	Path   string // JSON path, such as "minutes" or "calls[0].amount"
	Reason string
}

func (e *Error) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("%s: %s", e.Schema, e.Reason)
	}
	return fmt.Sprintf("%s: %s: %s", e.Schema, e.Path, e.Reason)
}

// Parse reads one JSON value, keeping numbers exact. As in core, an object
// with a duplicated key is an error, not "last one wins".
func Parse(data []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	v, err := readValue(d)
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing data after the JSON value")
	}
	return v, nil
}

func readValue(d *json.Decoder) (any, error) {
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		m := map[string]any{}
		for d.More() {
			kt, err := d.Token()
			if err != nil {
				return nil, err
			}
			k := kt.(string)
			if _, dup := m[k]; dup {
				return nil, fmt.Errorf("duplicate key %q", k)
			}
			if m[k], err = readValue(d); err != nil {
				return nil, err
			}
		}
		_, err := d.Token() // the closing brace
		return m, err
	case json.Delim('['):
		xs := []any{}
		for d.More() {
			x, err := readValue(d)
			if err != nil {
				return nil, err
			}
			xs = append(xs, x)
		}
		_, err := d.Token()
		return xs, err
	}
	return tok, nil
}

var (
	decimalPattern, _ = PatternByName("decimal")
	instantPattern, _ = PatternByName("instant")
)

// Reader walks a parsed document, recording the first error.
type Reader struct {
	Schema string
	Err    *Error
}

func (r *Reader) fail(path, format string, args ...any) {
	if r.Err == nil {
		r.Err = &Error{r.Schema, path, fmt.Sprintf(format, args...)}
	}
}

// Join adds a key to a path.
func Join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// Object reads a closed object: only the listed keys may appear.
func (r *Reader) Object(v any, path string, keys ...string) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		r.fail(path, "want an object")
		return nil
	}
	for k := range m {
		found := false
		for _, want := range keys {
			if k == want {
				found = true
				break
			}
		}
		if !found {
			r.fail(Join(path, k), "unknown field")
			return nil
		}
	}
	return m
}

// Field returns a required field.
func (r *Reader) Field(m map[string]any, path, key string) any {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		r.fail(Join(path, key), "missing")
	}
	return v
}

// Optional returns an optional field and whether it is present. A JSON null
// is present, and must have the field's type like any other value.
func Optional(m map[string]any, key string) (any, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[key]
	return v, ok
}

// Bool reads a boolean.
func (r *Reader) Bool(v any, path string) bool {
	b, ok := v.(bool)
	if !ok {
		r.fail(path, "want true or false")
	}
	return b
}

// IntBounds are an integer's limits from the schema.
type IntBounds struct {
	Min, Max   *int64
	MultipleOf int64
}

// Ptr is a helper for generated bounds.
func Ptr(n int64) *int64 { return &n }

func (b IntBounds) check(r *Reader, path string, n int64) {
	switch {
	case b.Min != nil && n < *b.Min:
		r.fail(path, "%d is below %d", n, *b.Min)
	case b.Max != nil && n > *b.Max:
		r.fail(path, "%d is above %d", n, *b.Max)
	case b.MultipleOf != 0 && n%b.MultipleOf != 0:
		r.fail(path, "%d is not a multiple of %d", n, b.MultipleOf)
	}
}

// Int reads a whole JSON number; 2.0 and 2e0 count.
func (r *Reader) Int(v any, path string, b IntBounds) int64 {
	num, ok := v.(json.Number)
	if !ok {
		r.fail(path, "want a whole number")
		return 0
	}
	rat, ok := new(big.Rat).SetString(string(num))
	if !ok || !rat.IsInt() || !rat.Num().IsInt64() {
		r.fail(path, "%s is not a whole number in range", num)
		return 0
	}
	n := rat.Num().Int64()
	b.check(r, path, n)
	return n
}

// StrBounds are a string's limits from the schema. Lengths count Unicode
// code points.
type StrBounds struct {
	MinLen, MaxLen *int64
	Pattern        string // a name from Patterns, or ""
}

// String reads a string.
func (r *Reader) String(v any, path string, b StrBounds) string {
	s, ok := v.(string)
	if !ok {
		r.fail(path, "want a string")
		return ""
	}
	n := int64(utf8.RuneCountInString(s))
	switch {
	case b.MinLen != nil && n < *b.MinLen:
		r.fail(path, "shorter than %d", *b.MinLen)
	case b.MaxLen != nil && n > *b.MaxLen:
		r.fail(path, "longer than %d", *b.MaxLen)
	case b.Pattern != "":
		p, ok := PatternByName(b.Pattern)
		if !ok || !p.Check(s) {
			r.fail(path, "%q is not a valid %s", s, b.Pattern)
		}
	}
	return s
}

// Enum reads one of a fixed set of strings.
func (r *Reader) Enum(v any, path string, values ...string) string {
	s, ok := v.(string)
	if ok {
		for _, w := range values {
			if s == w {
				return s
			}
		}
	}
	r.fail(path, "want one of %s", strings.Join(values, ", "))
	return ""
}

// Decimal reads a whole number written as a decimal string.
func (r *Reader) Decimal(v any, path string, b IntBounds) int64 {
	s, ok := v.(string)
	if !ok || !decimalPattern.Check(s) {
		r.fail(path, "want a whole number as a decimal string")
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		r.fail(path, "%s is out of range", s)
		return 0
	}
	b.check(r, path, n)
	return n
}

// Instant reads an RFC 3339 instant to the second, at any offset. Days past
// the end of a month roll over, as in core: 2026-02-30 is 2 March.
func (r *Reader) Instant(v any, path string) time.Time {
	s, ok := v.(string)
	if !ok || !instantPattern.Check(s) {
		r.fail(path, "want an RFC 3339 instant to the second")
		return time.Time{}
	}
	atoi := func(a, b int) int { n, _ := strconv.Atoi(s[a:b]); return n }
	t := time.Date(atoi(0, 4), time.Month(atoi(5, 7)), atoi(8, 10), atoi(11, 13), atoi(14, 16), atoi(17, 19), 0, time.UTC)
	if s[19] != 'Z' {
		off := time.Duration(atoi(20, 22))*time.Hour + time.Duration(atoi(23, 25))*time.Minute
		if s[19] == '+' {
			t = t.Add(-off)
		} else {
			t = t.Add(off)
		}
	}
	return t
}

// Tag reads the field that names a tagged union's case.
func (r *Reader) Tag(v any, path, tag string, cases ...string) string {
	m, ok := v.(map[string]any)
	if !ok {
		r.fail(path, "want an object")
		return ""
	}
	return r.Enum(m[tag], Join(path, tag), cases...)
}

// FormatInstant writes an instant in UTC to the second.
func FormatInstant(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") }

// FormatDecimal writes a whole number as a decimal string.
func FormatDecimal(n int64) string { return strconv.FormatInt(n, 10) }
