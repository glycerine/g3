package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestDualityLiteralClassificationAndBodyOnlyMaxIndex(t *testing.T) {
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
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	p, err := ctx.BoolConst("literal-p")
	must(err)
	closeExpr(t, p)
	yes := wrap(m.MkTrue())
	no := wrap(m.MkFalse())
	ph := raw(p)
	boolean, err := m.GetSort(ph)
	must(err)
	v0 := wrap(m.MkVar(0, boolean))
	v1 := wrap(m.MkVar(1, boolean))
	v3 := wrap(m.MkVar(3, boolean))
	quant := func(body *Expr, patterns []ast.ASTHandle) *Expr {
		return wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(body), Patterns: patterns}))
	}
	closed := quant(v0, nil)
	open := quant(v1, nil)
	triggerBody := wrap(m.MkOr(raw(v0), raw(v3)))
	trigger := wrap(m.MkPattern([]ast.ASTHandle{raw(triggerBody)}))
	annotated := quant(yes, []ast.ASTHandle{raw(trigger)})
	for _, test := range []struct {
		term  *Expr
		index int
	}{{p, -1}, {v3, 3}, {closed, -1}, {open, 0}, {annotated, -2}} {
		got, err := MaxIndex(test.term)
		must(err)
		if got != test.index {
			t.Fatal("source max index mismatch", got, test.index)
		}
		gotClosed, err := IsClosedFormula(test.term)
		must(err)
		if gotClosed != (test.index < 0) {
			t.Fatal("closed classification mismatch")
		}
	}
	np := wrap(m.MkNot(ph))
	nnp := wrap(m.MkNot(raw(np)))
	nclosed := wrap(m.MkNot(raw(closed)))
	and := wrap(m.MkAnd(ph, raw(yes)))
	or := wrap(m.MkOr(ph, raw(no)))
	iff := wrap(m.MkIff(ph, raw(yes)))
	implies := wrap(m.MkImplies(ph, raw(no)))
	for _, test := range []struct {
		term, atom *Expr
		positive   bool
	}{{p, p, true}, {yes, yes, true}, {np, p, false}, {nnp, p, true}, {closed, closed, true}, {nclosed, closed, false}, {annotated, annotated, true}, {open, nil, false}, {v0, nil, false}, {and, nil, false}, {or, nil, false}, {iff, nil, false}, {implies, nil, false}} {
		atom, positive, err := IsLiteral(test.term)
		must(err)
		if test.atom == nil {
			if atom != nil {
				must(atom.Close())
				t.Fatal("nonliteral accepted")
			}
			continue
		}
		if atom == nil {
			t.Fatal("literal rejected")
		}
		if raw(atom) != raw(test.atom) || positive != test.positive {
			t.Fatal("literal atom/polarity mismatch")
		}
		must(atom.Close())
	}
}

func TestDualityDeleteBoundMemoDepthAndQuantifierPatterns(t *testing.T) {
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
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	yes := wrap(m.MkTrue())
	boolean, err := m.GetSort(raw(yes))
	must(err)
	v0 := wrap(m.MkVar(0, boolean))
	v1 := wrap(m.MkVar(1, boolean))
	v2 := wrap(m.MkVar(2, boolean))
	body := wrap(m.MkOrArgs([]ast.ASTHandle{raw(v0), raw(v1), raw(v2)}))
	pattern := wrap(m.MkPattern([]ast.ASTHandle{raw(body)}))
	q := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(body), Patterns: []ast.ASTHandle{raw(pattern)}, Weight: 9}))
	root := wrap(m.MkAnd(raw(v1), raw(q)))
	result, err := DeleteBound(1, 1, root)
	must(err)
	defer func() { must(result.Close()) }()
	first, err := m.GetArg(raw(result), 0)
	must(err)
	if first != raw(v0) {
		t.Fatal("outer shared variable did not shift")
	}
	nested, err := m.GetArg(raw(result), 1)
	must(err)
	changedBody, err := m.GetQuantifierBody(nested)
	must(err)
	for i, want := range []ast.ASTHandle{raw(v0), raw(v1), raw(v1)} {
		child, err := m.GetArg(changedBody, uint32(i))
		must(err)
		if child != want {
			t.Fatal("binder-depth memo collision", i)
		}
	}
	changedPattern, err := m.GetPattern(nested, 0)
	must(err)
	patternBody, err := m.GetArg(changedPattern, 0)
	must(err)
	if patternBody != changedBody {
		t.Fatal("pattern did not follow body index shift")
	}
	weight, err := m.GetQuantifierWeight(nested)
	must(err)
	if weight != 9 {
		t.Fatal("quantifier metadata changed")
	}
	originalBody, err := m.GetQuantifierBody(raw(q))
	must(err)
	originalLast, err := m.GetArg(originalBody, 2)
	must(err)
	if originalLast != raw(v2) {
		t.Fatal("input expression mutated")
	}
	identity, err := DeleteBound(0, 0, root)
	must(err)
	defer identity.Close()
	if raw(identity) != raw(root) {
		t.Fatal("zero shift changed DAG")
	}
	if out, err := DeleteBound(0, 1, v0); err == nil {
		if out != nil {
			out.Close()
		}
		t.Fatal("accepted underflow outside source precondition")
	}
}
