// Package counter is clean: no diagnostics.
package counter

import "sort"

// Step applies one operation to the counter.
//
//tribrach:link Counter.counterStep
//tribrach:stamp 0123456789abcdef
func Step(op string, n, max int) int {
	if op == "reset" {
		return 0
	}
	if n < max {
		return next(n)
	}
	return n
}

//tribrach:pure
func next(n int) int { return n + 1 }

// Keys is pure and sorts what it reads from a map.
//
//tribrach:pure
func Keys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m { // want "TRG0306 Keys uses iteration over a map"
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Log isn't annotated, so it may do anything.
func Log() {}
