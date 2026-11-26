// Package counter implements the bounded counter from the Tribrach core
// example (examples/counter there). tribrach.manifest.json and
// testdata/counter.vectors.json are that example's exported files.
package counter

// Op is an operation on the counter.
type Op string

const (
	Incr  Op = "incr"
	Reset Op = "reset"
)

// Step applies one operation, never going above max.
//
//tribrach:link Counter.counterStep
//tribrach:stamp c5f9ea973150248a
func Step(max uint, op Op, n uint) uint {
	switch op {
	case Reset:
		return 0
	case Incr:
		if n < max {
			return n + 1
		}
	}
	return n
}
