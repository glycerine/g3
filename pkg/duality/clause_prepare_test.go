package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestDualityPrepareHornClauseFactsQueriesAndDelayedBinders(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
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
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer simplifier.Close()
	p, err := ctx.BoolConst("clause-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("clause-q")
	must(err)
	closeExpr(t, q)
	yes := wrap(m.MkTrue())
	no := wrap(m.MkFalse())
	rule := wrap(m.MkImplies(raw(p), raw(q)))
	query := wrap(m.MkImplies(raw(p), raw(no)))
	arbitrary := wrap(m.MkOr(raw(p), raw(q)))
	noncanonical := wrap(m.MkImplies(raw(p), raw(arbitrary)))
	for _, input := range []*Expr{p, rule, query, arbitrary, noncanonical, yes, no} {
		prepared, err := prepareHornClause(input, simplifier)
		must(err)
		if len(prepared.Replacements) != 0 {
			t.Fatal("ground clause gained substitutions")
		}
		normalized := wrap(m.MkImplies(raw(prepared.Body), raw(prepared.Head)))
		mismatch := wrap(m.MkNot(raw(wrap(m.MkIff(raw(input), raw(normalized))))))
		must(native.Push())
		must(native.Assert(raw(mismatch)))
		status, err := native.CheckSat(nil)
		must(err)
		must(native.Pop(1))
		if status != lbool.False {
			t.Fatal("clause normalization not equivalent")
		}
		if input == p && (raw(prepared.Body) != raw(yes) || raw(prepared.Head) != raw(p)) {
			t.Fatal("fact was not lifted")
		}
		if input == rule && (raw(prepared.Body) != raw(p) || raw(prepared.Head) != raw(q)) {
			t.Fatal("canonical rule changed")
		}
		if input == arbitrary || input == noncanonical {
			if raw(prepared.Head) != raw(no) {
				t.Fatal("noncanonical formula did not become query")
			}
		}
		must(prepared.close())
		must(prepared.close())
	}
	boolean, err := m.GetSort(raw(p))
	must(err)
	a, err := m.Symbols.Intern("clause-bound-a")
	must(err)
	b, err := m.Symbols.Intern("clause-bound-b")
	must(err)
	v0 := wrap(m.MkVar(0, boolean))
	v1 := wrap(m.MkVar(1, boolean))
	body := wrap(m.MkImplies(raw(v1), raw(q)))
	quantified := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean, boolean}, Names: []usymbol.Symbol{a, b}, Body: raw(body)}))
	prepared, err := prepareHornClause(quantified, simplifier)
	must(err)
	defer prepared.close()
	if raw(prepared.Body) != raw(v1) {
		t.Fatal("outer binder substituted before qe-lite")
	}
	if len(prepared.Replacements) != 2 {
		t.Fatal("missing delayed binder substitutions")
	}
	for index, want := range map[uint32]usymbol.Symbol{0: b, 1: a} {
		d, err := m.GetDecl(raw(prepared.Replacements[index]))
		must(err)
		name, err := m.GetASTName(d)
		must(err)
		if name != want {
			t.Fatal("de Bruijn substitution order reversed incorrectly")
		}
	}
	// Existentials are not stripped by the outer-forall conversion.
	exists := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{a}, Body: raw(v0)}))
	existential, err := prepareHornClause(exists, simplifier)
	must(err)
	defer existential.close()
	if len(existential.Replacements) != 0 || raw(existential.Head) != raw(no) {
		t.Fatal("existential treated as outer forall")
	}
}
