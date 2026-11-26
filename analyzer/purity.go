package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

// Purity codes and the model-side reason for each.
var reasons = map[string]string{
	"TRG0301": "the model has no clock: pass the time in as an argument",
	"TRG0302": "the model has no randomness: pass the random value or seed in as an argument",
	"TRG0303": "the model does no I/O: do it in the caller and pass the result in",
	"TRG0304": "the model has no shared mutable state: pass the value in or make it a constant",
	"TRG0305": "the model is sequential: keep goroutines, channels and locks in the caller",
	"TRG0306": "the model's lists have one order and Go map iteration has none: iterate over sorted keys",
	"TRG0307": "everything a linked function calls must be pure too: mark it //tribrach:pure or move the call out",
}

var clockFuncs = map[string]bool{
	"Now": true, "Since": true, "Until": true, "Sleep": true, "Tick": true,
	"After": true, "AfterFunc": true, "NewTimer": true, "NewTicker": true,
}

// ioPackages are packages whose functions all touch the outside world.
var ioPackages = map[string]bool{
	"os": true, "os/exec": true, "os/signal": true, "io/ioutil": true, "io/fs": true,
	"net": true, "net/http": true, "net/rpc": true, "net/smtp": true,
	"syscall": true, "log": true, "log/slog": true, "log/syslog": true,
	"database/sql": true, "plugin": true,
}

func classifyCall(fn *types.Func) string {
	if fn.Pkg() == nil {
		return ""
	}
	path, name := fn.Pkg().Path(), fn.Name()
	switch {
	case path == "time" && clockFuncs[name]:
		return "TRG0301"
	case path == "math/rand" || path == "math/rand/v2" || path == "crypto/rand":
		return "TRG0302"
	case ioPackages[path] || strings.HasPrefix(path, "net/"):
		return "TRG0303"
	case path == "fmt" && (strings.HasPrefix(name, "Print") || strings.HasPrefix(name, "Fprint") ||
		strings.HasPrefix(name, "Scan") || strings.HasPrefix(name, "Fscan")):
		return "TRG0303"
	case path == "sync" || path == "sync/atomic":
		return "TRG0305"
	}
	return ""
}

func funcName(fn *types.Func) string {
	if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
		return fn.FullName()
	}
	return fn.Pkg().Path() + "." + fn.Name()
}

func checkPurity(pass *analysis.Pass, l Linked, pure map[*types.Func]bool, allow map[string]bool,
	report func(token.Pos, string, string)) {
	subject := l.Decl.Name.Name
	if l.Lean != "" {
		subject = fmt.Sprintf("%s (linked to %s)", subject, l.Lean)
	}
	seen := map[string]bool{}
	flag := func(pos token.Pos, code, what string) {
		if allow[code] || allow[what] || seen[code+what] {
			return
		}
		seen[code+what] = true
		report(pos, code, fmt.Sprintf("%s uses %s; %s", subject, what, reasons[code]))
	}
	ast.Inspect(l.Decl.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			fn, ok := typeutil.Callee(pass.TypesInfo, x).(*types.Func)
			if !ok || fn.Pkg() == nil {
				return true
			}
			if code := classifyCall(fn); code != "" {
				flag(x.Pos(), code, funcName(fn))
			} else if fn.Pkg() == pass.Pkg && !pure[fn] && fn.Origin() == fn {
				if sig, ok := fn.Type().(*types.Signature); ok && !isInterfaceMethod(sig) {
					flag(x.Pos(), "TRG0307", funcName(fn))
				}
			}
		case *ast.Ident:
			if v, ok := pass.TypesInfo.Uses[x].(*types.Var); ok && v.Pkg() != nil && !v.IsField() &&
				v.Parent() == v.Pkg().Scope() {
				flag(x.Pos(), "TRG0304", "the package variable "+v.Pkg().Path()+"."+v.Name())
			}
		case *ast.GoStmt:
			flag(x.Pos(), "TRG0305", "a goroutine")
		case *ast.SendStmt:
			flag(x.Pos(), "TRG0305", "a channel send")
		case *ast.SelectStmt:
			flag(x.Pos(), "TRG0305", "select")
		case *ast.UnaryExpr:
			if x.Op == token.ARROW {
				flag(x.Pos(), "TRG0305", "a channel receive")
			}
		case *ast.RangeStmt:
			if t := pass.TypesInfo.TypeOf(x.X); t != nil {
				if _, ok := t.Underlying().(*types.Map); ok {
					flag(x.Pos(), "TRG0306", "iteration over a map")
				}
			}
		}
		return true
	})
}

func isInterfaceMethod(sig *types.Signature) bool {
	if sig.Recv() == nil {
		return false
	}
	return types.IsInterface(sig.Recv().Type())
}
