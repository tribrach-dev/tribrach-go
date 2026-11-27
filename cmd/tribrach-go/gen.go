package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tribrach-dev/tribrach-go/gen"
)

// genMain writes Go DTOs from JSON Schemas that core exported. Each argument
// is Name=path. With -n it writes nothing and exits 1 if the output file is
// out of date, which is how CI checks generated code.
func genMain(args []string) int {
	fs := flag.NewFlagSet("gen", flag.ExitOnError)
	pkg := fs.String("pkg", "", "Go package name for the generated file")
	out := fs.String("o", "", "output file")
	check := fs.Bool("n", false, "write nothing; exit 1 if the output file is out of date")
	fs.Parse(args)
	if *pkg == "" || *out == "" || fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: tribrach-go gen -pkg name -o file.go [-n] Name=schema.json ...")
		return 2
	}
	base := filepath.Dir(*out)
	var specs []gen.Spec
	for _, a := range fs.Args() {
		name, path, ok := strings.Cut(a, "=")
		if !ok || name == "" {
			fmt.Fprintf(os.Stderr, "tribrach-go gen: %q is not Name=schema.json\n", a)
			return 2
		}
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "tribrach-go gen:", err)
			return 2
		}
		src := path
		if rel, err := filepath.Rel(base, path); err == nil {
			src = filepath.ToSlash(rel)
		}
		specs = append(specs, gen.Spec{Name: name, Source: src, Schema: data})
	}
	code, err := gen.Generate(*pkg, specs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "TRG0402 tribrach-go gen:", err)
		return 2
	}
	if *check {
		have, _ := os.ReadFile(*out)
		if !bytes.Equal(have, code) {
			fmt.Fprintf(os.Stderr, "TRG0401 %s is out of date with the schemas core exported (Tribrach.Schema); fix: go generate, or run this command without -n\n", *out)
			return 1
		}
		return 0
	}
	if err := os.WriteFile(*out, code, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "tribrach-go gen:", err)
		return 2
	}
	return 0
}
