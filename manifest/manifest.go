// Package manifest reads the core formats tribrach.manifest.v1 and
// tribrach.vectors.v1.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	ManifestFormat = "tribrach.manifest.v1"
	VectorsFormat  = "tribrach.vectors.v1"
	// Owner is the instrument id whose links this package checks.
	Owner = "tribrach.go"
)

// Link ties a Lean definition to a host symbol.
type Link struct {
	Lean   string `json:"lean"`
	Owner  string `json:"owner"`
	Symbol string `json:"symbol"`
	Stamp  string `json:"stamp"`
}

// Manifest is a tribrach.manifest.v1 file.
type Manifest struct {
	Format  string `json:"format"`
	Package string `json:"package"`
	Links   []Link `json:"links"`
}

// Owned returns the links owned by owner.
func (m *Manifest) Owned(owner string) []Link {
	var out []Link
	for _, l := range m.Links {
		if l.Owner == owner {
			out = append(out, l)
		}
	}
	return out
}

// Load reads and checks a manifest file.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if m.Format != ManifestFormat {
		return nil, fmt.Errorf("%s: format %q, want %q (re-export it with this Tribrach version)", path, m.Format, ManifestFormat)
	}
	return &m, nil
}

// PackageOf returns the Go package path of a symbol such as
// "example.com/counter.Step" or "example.com/counter.Counter.Incr".
func PackageOf(symbol string) string {
	slash := strings.LastIndex(symbol, "/")
	dot := strings.Index(symbol[slash+1:], ".")
	if dot < 0 {
		return symbol
	}
	return symbol[:slash+1+dot]
}

// Case is one replay case.
type Case struct {
	ID     string            `json:"id"`
	Start  json.RawMessage   `json:"start"`
	Ops    []json.RawMessage `json:"ops"`
	States []json.RawMessage `json:"states"`
}

// Vectors is a tribrach.vectors.v1 file.
type Vectors struct {
	Format string `json:"format"`
	Model  string `json:"model"`
	Cases  []Case `json:"cases"`
}

// ParseVectors reads and checks vectors.
func ParseVectors(data []byte) (*Vectors, error) {
	var v Vectors
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	if v.Format != VectorsFormat {
		return nil, fmt.Errorf("format %q, want %q", v.Format, VectorsFormat)
	}
	for _, c := range v.Cases {
		if len(c.States) != len(c.Ops) {
			return nil, fmt.Errorf("case %s has %d operations but %d states", c.ID, len(c.Ops), len(c.States))
		}
	}
	return &v, nil
}

// LoadVectors reads and checks a vectors file.
func LoadVectors(path string) (*Vectors, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	v, err := ParseVectors(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return v, nil
}
