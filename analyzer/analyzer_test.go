package analyzer_test

import (
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/tribrach-dev/tribrach-go/analyzer"
)

func TestClean(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyzer.Analyzer, "example.com/counter")
}

func TestDrift(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyzer.Analyzer, "example.com/drifty")
}

func TestPurity(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyzer.Analyzer, "example.com/impure")
}

// Directive problems sit on comment lines, which can't also carry a want
// comment, so they are checked here.
func TestCollect(t *testing.T) {
	src := `package p

//tribrach:link A.b C.d
func Bad() {}

func Inside() {
	//tribrach:pure
}

//tribrach:link A.one
//tribrach:link A.two
func Twice() {}

//tribrach:link A.m
//tribrach:stamp 0123456789abcdef
func (r *T[K]) M() {}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	got := analyzer.Collect("example.com/p", f, func(_ token.Pos, code, _ string) { codes = append(codes, code) })
	want := []string{"TRG0101", "TRG0102", "TRG0103"}
	if len(codes) != len(want) {
		t.Fatalf("codes = %v, want %v", codes, want)
	}
	for i := range want {
		if codes[i] != want[i] {
			t.Errorf("codes = %v, want %v", codes, want)
		}
	}
	syms := map[string]string{}
	for _, l := range got {
		syms[l.Symbol] = l.Lean + "@" + l.Stamp
	}
	if syms["example.com/p.T.M"] != "A.m@0123456789abcdef" {
		t.Errorf("method symbol: %v", syms)
	}
	if syms["example.com/p.Twice"] != "A.one@" {
		t.Errorf("twice: %v", syms)
	}
}
