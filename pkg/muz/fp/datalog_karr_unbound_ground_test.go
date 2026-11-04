package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/math/hilbert"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogKarrUnboundNoTailPublicAndRetry(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(err)
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "karr_relation"))
	must(g.SourceSetName("fixedpoint.xform.karr", "true"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	decl := func(name string) ast.ASTHandle {
		sym, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer, integer, integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p := decl("karr-unbound-ground")
	x, err := m.MkVar(0, integer)
	must(err)
	four, err := m.ArithmeticIntegerNumeralText("4")
	must(err)
	head, err := m.MkApp(p, []ast.ASTHandle{x, four, x})
	must(err)
	must(e.AddRule(head, ""))
	status, err := e.QueryPredicate(p)
	must(err)
	if status != lbool.Undef {
		t.Fatal("status", status)
	}
	borrowed, err := e.set.Rule(0)
	must(err)
	_, plan, err := e.karrDatalog.compileGroundRule(borrowed)
	must(err)
	must(borrowed.Close())
	if len(plan.values) != 2 || plan.values[0] != nil || len(plan.head.duplicates) != 1 || plan.head.duplicates[0] != 0 {
		t.Fatal("unbound compilation", plan)
	}
	fired := false
	_, err = plan.apply(func() bool { fired = true; m.Limit.Cancel(); return false })
	must(err)
	if !fired || e.karrDatalog.checkpoint() == nil {
		t.Fatal("duplicate projection skipped cancellation")
	}
	m.Limit.ResetCancel()
	status, err = e.QueryPredicate(p)
	must(err)
	if status != lbool.Undef {
		t.Fatal("retry after interrupted assembly", status)
	}
	direct, err := plan.apply(nil)
	must(err)
	matrix, err := direct.Inequalities()
	must(err)
	if direct.Dimension() != 3 || direct.Empty() || matrix.Len() < 2 {
		t.Fatal("unbound assembly", matrix)
	}
	answer, err := e.Answer(p)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	h, err := m.RefGet(answer)
	must(err)
	subst, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(subst.Close()) }()
	check := func(values []string, want bool) {
		args := make([]ast.ASTHandle, len(values))
		for i, v := range values {
			args[i], err = m.ArithmeticIntegerNumeralText(v)
			must(err)
		}
		grounded, err := m.NewRef(ast.ASTHandle{})
		must(err)
		defer func() { must(m.DelRef(&grounded)) }()
		must(subst.Substitute(h, args, grounded))
		expr, err := m.RefGet(grounded)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		defer func() { must(m.DelRef(&reduced)) }()
		must(e.guard.RewriteWithBindings(expr, nil, reduced))
		expr, err = m.RefGet(reduced)
		must(err)
		if (want && !m.IsTrue(expr)) || (!want && !m.IsFalse(expr)) {
			t.Fatal("answer", values, want, expr)
		}
	}
	check([]string{"-7", "4", "-7"}, true)
	check([]string{"42", "4", "42"}, true)
	check([]string{"42", "4", "41"}, false)
	check([]string{"42", "3", "42"}, false)
	status, err = e.QueryExpression(head)
	must(err)
	if status != lbool.Undef || e.queryClone == nil || e.queryClone.karrDatalog == e.karrDatalog {
		t.Fatal("expression clone", status)
	}
	held, err := e.ExpressionAnswer()
	must(err)
	must(e.clearQueryResources())
	if _, err = m.RefGet(held); err != nil {
		t.Fatal("expression answer lifetime", err)
	}
	must(m.DelRef(&held))
	m.Limit.Cancel()
	status, err = e.QueryPredicate(p)
	m.Limit.ResetCancel()
	if status != lbool.Undef || err == nil || e.karrDatalog.results != nil {
		t.Fatal("cancel", status, err)
	}
	status, err = e.QueryPredicate(p)
	must(err)
	if status != lbool.Undef {
		t.Fatal("retry", status)
	}
}

func TestDatalogKarrUnboundGroundOverflowNoPublication(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(err)
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "karr_relation"))
	must(g.SourceSetName("fixedpoint.xform.karr", "true"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	sym, err := m.Symbols.Intern("karr-unbound-overflow")
	must(err)
	p, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(p))
	x, err := m.MkVar(0, integer)
	must(err)
	huge, err := m.ArithmeticIntegerNumeralText("9223372036854775808")
	must(err)
	head, err := m.MkApp(p, []ast.ASTHandle{x, huge})
	must(err)
	must(e.AddRule(head, ""))
	for i := 0; i < 2; i++ {
		status, err := e.QueryPredicate(p)
		if status != lbool.Undef || !errors.Is(err, hilbert.ErrOverflow) || e.karrDatalog.results != nil {
			t.Fatal("overflow", status, err)
		}
	}
	rule, err := e.set.Rule(0)
	must(err)
	must(e.set.Remove(rule))
	must(rule.Close())
	status, err := e.karrDatalog.query([]ast.ASTHandle{p})
	must(err)
	if status != lbool.False {
		t.Fatal("overflow recovery", status)
	}
}

func TestDatalogKarrWhollyUnboundNativeBasisAndRepeatedVariables(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(err)
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "karr_relation"))
	must(g.SourceSetName("fixedpoint.xform.karr", "true"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	decl := func(name string, n int) ast.ASTHandle {
		sym, err := m.Symbols.Intern(name)
		must(err)
		sorts := make([]ast.ASTHandle, n)
		for i := range sorts {
			sorts[i] = integer
		}
		d, err := m.MkFuncDecl(sym, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	full, seeded, repeated := decl("unbound-full", 2), decl("unbound-seeded", 2), decl("unbound-repeated", 4)
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	add := func(d ast.ASTHandle, args ...ast.ASTHandle) {
		h, err := m.MkApp(d, args)
		must(err)
		must(e.AddRule(h, ""))
	}
	must(e.AddTableFact(seeded, []uint32{1, 2}))
	add(full, x, y)
	add(seeded, x, y)
	add(repeated, y, x, y, x)
	for _, d := range []ast.ASTHandle{full, seeded, repeated} {
		status, err := e.QueryPredicate(d)
		must(err)
		if status != lbool.Undef {
			t.Fatal("status", status)
		}
	}
	matrix, err := e.karrDatalog.results[full].Basis()
	must(err)
	if matrix.Len() != 0 {
		t.Fatal("native full relation basis must be empty", matrix)
	}
	seededBasis, err := e.karrDatalog.results[seeded].Basis()
	must(err)
	if seededBasis.Len() != 1 {
		t.Fatal("native union of empty basis must preserve seed", seededBasis)
	}
	repeatedIneq, err := e.karrDatalog.results[repeated].Inequalities()
	must(err)
	if repeatedIneq.Len() != 4 {
		t.Fatal("two independent repeated equalities", repeatedIneq)
	}
	subst, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(subst.Close()) }()
	check := func(d ast.ASTHandle, values []string, want bool) {
		answer, err := e.Answer(d)
		must(err)
		defer func() { must(m.DelRef(&answer)) }()
		expr, err := m.RefGet(answer)
		must(err)
		args := make([]ast.ASTHandle, len(values))
		for i, v := range values {
			args[i], err = m.ArithmeticIntegerNumeralText(v)
			must(err)
		}
		grounded, err := m.NewRef(ast.ASTHandle{})
		must(err)
		defer func() { must(m.DelRef(&grounded)) }()
		must(subst.Substitute(expr, args, grounded))
		expr, err = m.RefGet(grounded)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		defer func() { must(m.DelRef(&reduced)) }()
		must(e.guard.RewriteWithBindings(expr, nil, reduced))
		expr, err = m.RefGet(reduced)
		must(err)
		if (want && !m.IsTrue(expr)) || (!want && !m.IsFalse(expr)) {
			t.Fatal("unbound answer", values, want, expr)
		}
	}
	check(full, []string{"9", "8"}, true)
	check(seeded, []string{"1", "2"}, true)
	check(seeded, []string{"9", "8"}, false)
	check(repeated, []string{"9", "8", "9", "8"}, true)
	check(repeated, []string{"9", "8", "8", "9"}, false)
}
