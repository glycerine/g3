package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestUniformModelBoolAgainstGroundBindings(t *testing.T) {
	for _, name := range []string{"missing", "always_true", "always_false", "exception"} {
		t.Run(name, func(t *testing.T) { checkUniformGroundBindings(t, name) })
	}
}

func checkUniformGroundBindings(t *testing.T, name string) {
	want := lbool.Undef
	m, _, md, _, _ := smtAuxiliaryQueryFixture(t, false)
	f := tableTest{t, m, md}
	boolean := f.term(m.MkBoolSort())
	x := f.term(m.MkVar(0, boolean))
	yes, no := f.term(m.MkTrue()), f.term(m.MkFalse())
	decl := smtQuickPredicate(t, m, "uniform", boolean, boolean)
	p := f.term(m.MkApp(decl, []ast.ASTHandle{x}))
	tab, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(t, err)
	astTestMust(t, md.Core().RegisterFunction(decl, tab))
	forms := []ast.ASTHandle{p, x}
	for _, a := range []ast.ASTHandle{p, x, no, yes} {
		forms = append(forms, f.term(m.MkNot(a)))
		for _, b := range []ast.ASTHandle{p, x, no, yes} {
			forms = append(forms,
				f.term(m.MkIff(a, b)), f.term(m.MkXor(a, b)), f.term(m.MkImplies(a, b)),
				f.term(m.MkOrArgs([]ast.ASTHandle{a, b})), f.term(m.MkAndArgs([]ast.ASTHandle{a, b})),
				f.term(m.MkIte(x, a, b)),
			)
		}
	}
	switch name {
	case "always_true":
		want = lbool.True
		astTestMust(t, tab.SetElse(yes))
	case "always_false":
		want = lbool.False
		astTestMust(t, tab.SetElse(no))
	case "exception":
		astTestMust(t, tab.SetElse(no))
		astTestMust(t, tab.InsertNewEntry([]ast.ASTHandle{no}, yes))
	}
	fuel := 96
	if got := uniformModelBool(m, md, p, boolean, &fuel); got != want {
		t.Fatalf("predicate in model %s: %v, want %v", name, got, want)
	}
	if name == "missing" {
		return
	}
	for _, body := range forms {
		fuel = 96
		got := uniformModelBool(m, md, body, boolean, &fuel)
		if got == lbool.Undef {
			continue
		}
		q := f.forall(body, boolean)
		for _, v := range []ast.ASTHandle{no, yes} {
			out, e := m.NewRef(ast.ASTHandle{})
			astTestMust(t, e)
			astTestMust(t, rewriter.InstantiateQuantifier(m, q, []ast.ASTHandle{v}, out))
			ground, e := m.RefGet(out)
			astTestMust(t, e)
			ok, e := md.Eval(ground, out, true)
			astTestMust(t, e)
			value, e := m.RefGet(out)
			astTestMust(t, e)
			if !ok || got == lbool.True && value != yes || got == lbool.False && value != no {
				t.Fatalf("model %s, body %v, binding %v: uniform %v, ground %v", name, body, v, got, value)
			}
			astTestMust(t, m.DelRef(&out))
		}
	}
}

func BenchmarkUniformPredicateModelCheck(b *testing.B) {
	m, _, md, _, _ := smtAuxiliaryQueryFixture(b, false)
	f := tableTest{b, m, md}
	boolean := f.term(m.MkBoolSort())
	domain := f.term(m.MkUninterpretedSort(astTestSymbol(b, m, "uniform-domain"), nil))
	for i := 0; i < 8; i++ {
		astTestMust(b, md.RegisterValue(f.term(m.MkModelValue(uint32(i), domain))))
	}
	astTestMust(b, md.FreezeUniverse(domain))
	pred := smtQuickPredicate(b, m, "uniform-predicate", domain, boolean)
	tab, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(b, err)
	astTestMust(b, tab.SetElse(f.term(m.MkTrue())))
	astTestMust(b, md.Core().RegisterFunction(pred, tab))
	x, y := f.term(m.MkVar(0, domain)), f.term(m.MkVar(1, domain))
	body := f.term(m.MkOrArgs([]ast.ASTHandle{f.term(m.MkEq(x, y)), f.term(m.MkApp(pred, []ast.ASTHandle{x}))}))
	q := f.forall(body, domain, domain)
	var scratch tableModel
	b.ReportAllocs()
	for b.Loop() {
		status, _, err := scratch.check(m, md, q, body, 2, nil)
		if err != nil || status != lbool.False {
			b.Fatalf("check: %v, %v", status, err)
		}
	}
}
