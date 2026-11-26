// Package analyzer is the tribrach go vet analyzer: links, purity and drift
// for Go functions annotated with //tribrach: directives.
//
// Run it with go vet:
//
//	go install github.com/tribrach-dev/tribrach-go/cmd/tribrach-vet@latest
//	go vet -vettool=$(which tribrach-vet) ./...
//
// The manifest is the nearest tribrach.manifest.json above each package's
// directory, or the file named by -manifest.
package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"

	"github.com/tribrach-dev/tribrach-go/annotation"
	"github.com/tribrach-dev/tribrach-go/drift"
	"github.com/tribrach-dev/tribrach-go/manifest"
)

// Analyzer is the tribrach analyzer.
var Analyzer = &analysis.Analyzer{
	Name: "tribrach",
	Doc:  "check //tribrach: links, purity and drift against the Lean manifest",
	Run:  run,
}

var (
	manifestFlag string
	allowFlag    string
)

func init() {
	Analyzer.Flags.StringVar(&manifestFlag, "manifest", "", "path to tribrach.manifest.json (default: nearest one above each package)")
	Analyzer.Flags.StringVar(&allowFlag, "allow", "", "comma-separated diagnostic codes (TRG0306) or functions (time.Now) to allow in pure code")
}

// ManifestName is the file the analyzer looks for above each package.
const ManifestName = "tribrach.manifest.json"

var manifests sync.Map // path -> *loaded

type loaded struct {
	m   *manifest.Manifest
	err error
}

func loadManifest(path string) (*manifest.Manifest, error) {
	if v, ok := manifests.Load(path); ok {
		l := v.(*loaded)
		return l.m, l.err
	}
	m, err := manifest.Load(path)
	manifests.Store(path, &loaded{m, err})
	return m, err
}

func findManifest(dir string) string {
	if manifestFlag != "" {
		return manifestFlag
	}
	for {
		p := filepath.Join(dir, ManifestName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// Linked is one annotated function.
type Linked struct {
	Decl   *ast.FuncDecl
	Symbol string
	Lean   string // empty for //tribrach:pure without a link
	Stamp  string
	Pure   bool // explicitly //tribrach:pure, or linked (linked implies pure)
	Pos    token.Pos
}

// Symbol returns the manifest notation for a function: pkgpath.Func or
// pkgpath.Type.Method.
func Symbol(pkgPath string, fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return pkgPath + "." + fd.Name.Name
	}
	t := fd.Recv.List[0].Type
	for {
		switch x := t.(type) {
		case *ast.StarExpr:
			t = x.X
			continue
		case *ast.IndexExpr:
			t = x.X
			continue
		case *ast.IndexListExpr:
			t = x.X
			continue
		case *ast.ParenExpr:
			t = x.X
			continue
		}
		break
	}
	name := "?"
	if id, ok := t.(*ast.Ident); ok {
		name = id.Name
	}
	return pkgPath + "." + name + "." + fd.Name.Name
}

// Collect reads the directives in one file. report is called for malformed
// or misplaced directives with a code and message.
func Collect(pkgPath string, f *ast.File, report func(pos token.Pos, code, msg string)) []Linked {
	docs := map[*ast.CommentGroup]*ast.FuncDecl{}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Doc != nil {
			docs[fd.Doc] = fd
		}
	}
	var out []Linked
	for _, cg := range f.Comments {
		fd := docs[cg]
		var l Linked
		any := false
		for _, c := range cg.List {
			d, ok := annotation.Parse(c.Text)
			if !ok {
				continue
			}
			if d.Problem != "" {
				report(c.Pos(), "TRG0101", fmt.Sprintf("can't read %q: %s; the forms are //tribrach:link <LeanName>, //tribrach:pure and //tribrach:stamp <16 hex digits>", c.Text, d.Problem))
				continue
			}
			if fd == nil {
				report(c.Pos(), "TRG0102", fmt.Sprintf("%q isn't attached to a function: put it in the function's doc comment", c.Text))
				continue
			}
			any = true
			switch d.Kind {
			case annotation.Link:
				if l.Lean != "" {
					report(c.Pos(), "TRG0103", fmt.Sprintf("%s already links %s; a function implements one Lean definition", fd.Name.Name, l.Lean))
					continue
				}
				l.Lean, l.Pos, l.Pure = d.Lean, c.Pos(), true
			case annotation.Stamp:
				l.Stamp = d.Stamp
				if l.Pos == token.NoPos {
					l.Pos = c.Pos()
				}
			case annotation.Pure:
				l.Pure = true
				if l.Pos == token.NoPos {
					l.Pos = c.Pos()
				}
			}
		}
		if !any {
			continue
		}
		if l.Stamp != "" && l.Lean == "" {
			report(fd.Name.Pos(), "TRG0104", fmt.Sprintf("%s has a //tribrach:stamp but no //tribrach:link; a stamp records which Lean definition it was checked against", fd.Name.Name))
		}
		l.Decl = fd
		l.Symbol = Symbol(pkgPath, fd)
		out = append(out, l)
	}
	return out
}

func run(pass *analysis.Pass) (any, error) {
	report := func(pos token.Pos, code, msg string) {
		pass.Report(analysis.Diagnostic{Pos: pos, Category: code, Message: code + " " + msg})
	}
	var linked []Linked
	for _, f := range pass.Files {
		linked = append(linked, Collect(pass.Pkg.Path(), f, report)...)
	}
	allow := parseAllow(allowFlag)
	pureFuncs := map[*types.Func]bool{}
	for _, l := range linked {
		if obj, ok := pass.TypesInfo.Defs[l.Decl.Name].(*types.Func); ok && l.Pure {
			pureFuncs[obj] = true
		}
	}
	for _, l := range linked {
		if l.Pure && l.Decl.Body != nil {
			checkPurity(pass, l, pureFuncs, allow, report)
		}
	}
	checkDrift(pass, linked, report)
	return nil, nil
}

func checkDrift(pass *analysis.Pass, linked []Linked, report func(token.Pos, string, string)) {
	if len(pass.Files) == 0 {
		return
	}
	dir := filepath.Dir(pass.Fset.File(pass.Files[0].Pos()).Name())
	path := findManifest(dir)
	var hosts []drift.Host
	where := map[[2]string]Linked{}
	for _, l := range linked {
		if l.Lean == "" {
			continue
		}
		hosts = append(hosts, drift.Host{Symbol: l.Symbol, Lean: l.Lean, Stamp: l.Stamp})
		where[[2]string{l.Lean, l.Symbol}] = l
	}
	if path == "" {
		if len(hosts) > 0 {
			report(where[[2]string{hosts[0].Lean, hosts[0].Symbol}].Decl.Name.Pos(), "TRG0200",
				"no "+ManifestName+" found above this package: export one from Lean or pass -manifest")
		}
		return
	}
	m, err := loadManifest(path)
	if err != nil {
		report(pass.Files[0].Package, "TRG0200", "can't read the manifest: "+err.Error())
		return
	}
	// Only this package's manifest links can be missing here.
	var links []manifest.Link
	for _, l := range m.Owned(manifest.Owner) {
		if manifest.PackageOf(l.Symbol) == pass.Pkg.Path() {
			links = append(links, l)
		} else {
			for _, h := range hosts {
				if h.Lean == l.Lean {
					links = append(links, l)
					break
				}
			}
		}
	}
	for _, f := range drift.Compare(manifest.Owner, links, hosts) {
		l := where[[2]string{f.Lean, f.Symbol}]
		pos := pass.Files[0].Package
		if l.Decl != nil {
			pos = l.Decl.Name.Pos()
		}
		switch f.Outcome {
		case drift.Drift:
			report(pos, "TRG0201", fmt.Sprintf("%s changed in Lean (stamp %s, recorded %s): check %s against the model, run the vectors, then run `tribrach-go stamp`", f.Lean, f.Want, l.Stamp, l.Decl.Name.Name))
		case drift.Unstamped:
			report(pos, "TRG0202", fmt.Sprintf("%s links %s but records no stamp: once it matches the model, run `tribrach-go stamp` (it will record %s)", l.Decl.Name.Name, f.Lean, f.Want))
		case drift.Unknown:
			report(pos, "TRG0203", fmt.Sprintf("the manifest (%s) has no Go link for %s: check the Lean name, or add a link to the exporter and re-export", filepath.Base(path), f.Lean))
		case drift.SymbolMismatch:
			report(pos, "TRG0204", fmt.Sprintf("the manifest links %s to %s, not %s: update the exporter's symbol and re-export, or move the annotation", f.Lean, f.Want, f.Symbol))
		case drift.Missing:
			report(pass.Files[0].Package, "TRG0205", fmt.Sprintf("the manifest links %s to %s, but no function here has //tribrach:link %s", f.Lean, f.Symbol, f.Lean))
		}
	}
}

func parseAllow(s string) map[string]bool {
	out := map[string]bool{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out[p] = true
		}
	}
	return out
}
