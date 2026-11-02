package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestDualityScanBodyRepeatedPredicatesAndQuantifierPatterns(t *testing.T) {
	ctx, m, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		e, err := NewExpr(ctx, h)
		must(err)
		closeExpr(t, e)
		return e
	}
	raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
	yes := wrap(m.MkTrue())
	boolean, err := m.GetSort(raw(yes))
	must(err)
	v0 := wrap(m.MkVar(0, boolean))
	v1 := wrap(m.MkVar(1, boolean))
	name, err := m.Symbols.Intern("scan-p")
	must(err)
	pd, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	// Pin the declaration through applications before building the graph.
	p0 := wrap(m.MkApp(pd, []ast.ASTHandle{raw(v0)}))
	p1 := wrap(m.MkApp(pd, []ast.ASTHandle{raw(v1)}))
	g, err := NewRPFP(ctx)
	must(err)
	defer g.Close()
	node, err := g.CreateNode(p0)
	must(err)
	pattern := wrap(m.MkPattern([]ast.ASTHandle{raw(p0)}))
	q := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(p0), Patterns: []ast.ASTHandle{raw(pattern)}, Weight: 5}))
	root := wrap(m.MkAndArgs([]ast.ASTHandle{raw(p0), raw(p1), raw(p0), raw(q), raw(yes)}))
	scanned, err := g.ScanBody(root, map[ast.ASTHandle]*Node{pd: node})
	must(err)
	defer func() { must(scanned.Close()); must(scanned.Close()) }()
	if len(scanned.Parameters) != 2 || len(scanned.Children) != 2 {
		t.Fatal("wrong positional relation count")
	}
	for i, p := range scanned.Parameters {
		if scanned.Children[i] != node {
			t.Fatal("repeated predicate child lost")
		}
		name, err := p.Name()
		must(err)
		text, err := name.String()
		must(err)
		want := []string{"scan-p_0", "scan-p_1"}[i]
		if text != want {
			t.Fatalf("name %q want %q", text, want)
		}
		arity, err := p.Arity()
		must(err)
		if arity != 1 {
			t.Fatal("signature changed")
		}
	}
	h := raw(scanned.Term)
	first, err := m.GetArg(h, 0)
	must(err)
	second, err := m.GetArg(h, 1)
	must(err)
	repeated, err := m.GetArg(h, 2)
	must(err)
	if first != repeated || first == second {
		t.Fatal("application memoization mismatch")
	}
	for i, app := range []ast.ASTHandle{first, second} {
		decl, err := m.GetDecl(app)
		must(err)
		want, err := scanned.Parameters[i].Raw()
		must(err)
		if decl != want {
			t.Fatal("parameter order mismatch")
		}
		arg, err := m.GetArg(app, 0)
		must(err)
		if arg != []ast.ASTHandle{raw(v0), raw(v1)}[i] {
			t.Fatal("predicate argument changed")
		}
	}
	quant, err := m.GetArg(h, 3)
	must(err)
	body, err := m.GetQuantifierBody(quant)
	must(err)
	if body != first {
		t.Fatal("shared quantified application not memoized")
	}
	pat, err := m.GetPattern(quant, 0)
	must(err)
	if pat != raw(pattern) {
		t.Fatal("pattern changed")
	}
	original, err := m.GetArg(raw(root), 0)
	must(err)
	if original != raw(p0) {
		t.Fatal("source mutated")
	}
	// A missing predicate map leaves the original formula and background alone.
	untouched, err := g.ScanBody(root, nil)
	must(err)
	defer untouched.Close()
	if raw(untouched.Term) != raw(root) || len(untouched.Parameters) != 0 || len(g.vocabulary.constants) != 0 {
		t.Fatal("background formula changed")
	}
	// The production-only abstraction is deliberately limited to a positive
	// top-level forall=true premise. The same wrapper below Not must remain an
	// ordinary RPFP relation occurrence.
	wrapper := wrap(m.MkEq(raw(q), raw(yes)))
	positive, err := g.scanBodyTheoryQuantifiers(wrapper, map[ast.ASTHandle]*Node{pd: node})
	must(err)
	defer positive.Close()
	if !positive.QuantifiedTheory || len(positive.Parameters) != 0 || len(positive.Children) != 0 {
		t.Fatal("positive quantified wrapper was not kept as a theory obligation")
	}
	negated := wrap(m.MkNot(raw(wrapper)))
	negative, err := g.scanBodyTheoryQuantifiers(negated, map[ast.ASTHandle]*Node{pd: node})
	must(err)
	defer negative.Close()
	if negative.QuantifiedTheory || len(negative.Parameters) != 1 || len(negative.Children) != 1 {
		t.Fatal("nonpositive quantified wrapper was generalized")
	}
	must(g.Close())
	if _, err := scanned.Term.Raw(); err != nil {
		t.Fatal("output lost ownership", err)
	}
}
