// Package tribrach is the Go instrument for Tribrach.
//
// It links Go functions to Lean definitions with //tribrach:link comments,
// keeps linked code pure, reports drift between the Go code and the Lean
// manifest, and replays exported vectors against the Go implementation.
// See https://github.com/tribrach-dev/tribrach for the formats it reads.
package tribrach
