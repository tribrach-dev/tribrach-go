// Command tribrach-go records Lean stamps next to linked Go functions and
// generates Go DTOs from core's JSON Schemas.
//
//	tribrach-go stamp [-manifest path] [-n] [packages]
//	tribrach-go gen -pkg name -o file.go [-n] Name=schema.json ...
//
// stamp writes or updates the //tribrach:stamp line under each
// //tribrach:link so it matches the manifest. Run it deliberately, after the
// function has been checked against the model and the vectors pass: the
// stamp records that check. With -n it changes nothing, lists the stale
// stamps and exits 1 if there are any.
package main

import (
	"flag"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/tools/go/packages"

	"github.com/tribrach-dev/tribrach-go/analyzer"
	"github.com/tribrach-dev/tribrach-go/manifest"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "gen" {
		os.Exit(genMain(os.Args[2:]))
	}
	if len(os.Args) < 2 || os.Args[1] != "stamp" {
		fmt.Fprintln(os.Stderr, "usage: tribrach-go stamp [-manifest path] [-n] [packages]")
		fmt.Fprintln(os.Stderr, "       tribrach-go gen -pkg name -o file.go [-n] Name=schema.json ...")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("stamp", flag.ExitOnError)
	manifestPath := fs.String("manifest", "", "path to tribrach.manifest.json (default: nearest one above the current directory)")
	dryRun := fs.Bool("n", false, "list stale stamps without changing files; exit 1 if any")
	fs.Parse(os.Args[2:])
	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	code, err := stamp(*manifestPath, *dryRun, patterns)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tribrach-go:", err)
		os.Exit(2)
	}
	os.Exit(code)
}

func nearestManifest() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		p := filepath.Join(dir, analyzer.ManifestName)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s above the current directory; pass -manifest", analyzer.ManifestName)
		}
		dir = parent
	}
}

type edit struct {
	start, end int // byte offsets; start == end inserts
	text       string
}

func stamp(manifestPath string, dryRun bool, patterns []string) (int, error) {
	if manifestPath == "" {
		p, err := nearestManifest()
		if err != nil {
			return 0, err
		}
		manifestPath = p
	}
	m, err := manifest.Load(manifestPath)
	if err != nil {
		return 0, err
	}
	want := map[[2]string]string{}
	for _, l := range m.Owned(manifest.Owner) {
		want[[2]string{l.Lean, l.Symbol}] = l.Stamp
	}
	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax,
		Fset: fset,
	}, patterns...)
	if err != nil {
		return 0, err
	}
	edits := map[string][]edit{}
	stale := 0
	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			for _, l := range analyzer.Collect(pkg.PkgPath, f, func(token.Pos, string, string) {}) {
				if l.Lean == "" {
					continue
				}
				s, ok := want[[2]string{l.Lean, l.Symbol}]
				if !ok || s == l.Stamp {
					continue
				}
				stale++
				pos := fset.Position(l.Decl.Name.Pos())
				fmt.Printf("%s:%d: %s %s -> %s\n", pos.Filename, pos.Line, l.Symbol, orNone(l.Stamp), s)
				e, err := stampEdit(fset, l, s)
				if err != nil {
					return 0, err
				}
				edits[pos.Filename] = append(edits[pos.Filename], e)
			}
		}
	}
	if dryRun {
		if stale > 0 {
			return 1, nil
		}
		return 0, nil
	}
	for name, es := range edits {
		if err := apply(name, es); err != nil {
			return 0, err
		}
	}
	return 0, nil
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// stampEdit replaces the existing stamp line, or inserts one after the link.
func stampEdit(fset *token.FileSet, l analyzer.Linked, s string) (edit, error) {
	line := "//tribrach:stamp " + s
	for _, c := range l.Decl.Doc.List {
		if len(c.Text) >= len("//tribrach:stamp") && c.Text[:len("//tribrach:stamp")] == "//tribrach:stamp" {
			p := fset.Position(c.Pos())
			return edit{p.Offset, p.Offset + len(c.Text), line}, nil
		}
	}
	for _, c := range l.Decl.Doc.List {
		if len(c.Text) >= len("//tribrach:link") && c.Text[:len("//tribrach:link")] == "//tribrach:link" {
			p := fset.Position(c.Pos())
			end := p.Offset + len(c.Text)
			indent := make([]byte, p.Column-1)
			for i := range indent {
				indent[i] = '\t'
			}
			return edit{end, end, "\n" + string(indent) + line}, nil
		}
	}
	return edit{}, fmt.Errorf("%s: no //tribrach:link line found", l.Symbol)
}

func apply(name string, es []edit) error {
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	sort.Slice(es, func(i, j int) bool { return es[i].start > es[j].start })
	for _, e := range es {
		data = append(data[:e.start], append([]byte(e.text), data[e.end:]...)...)
	}
	return os.WriteFile(name, data, 0o644)
}
