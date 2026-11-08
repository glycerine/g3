package fp

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/math/hilbert"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogKarrUnboundUnaryAndBinaryHead(t *testing.T) {
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
	p, q, u, b := decl("unbound-head-p", 1), decl("unbound-head-q", 1), decl("unbound-head-unary", 4), decl("unbound-head-binary", 4)
	must(e.AddTableFact(p, []uint32{1}))
	must(e.AddTableFact(q, []uint32{3}))
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	z, err := m.MkVar(2, integer)
	must(err)
	four, err := m.ArithmeticIntegerNumeralText("4")
	must(err)
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(d, args)
		must(err)
		return h
	}
	unary, err := m.MkImplies(app(p, x), app(u, y, x, four, y))
	must(err)
	must(e.addRuleEager(unary, ""))
	body, err := m.MkAnd(app(p, x), app(q, z))
	must(err)
	binary, err := m.MkImplies(body, app(b, y, z, y, x))
	must(err)
	must(e.addRuleEager(binary, ""))
	for _, d := range []ast.ASTHandle{u, b} {
		status, err := e.QueryPredicate(d)
		must(err)
		if status != lbool.Undef {
			t.Fatal("status", status)
		}
	}
	// The source adds a total column for y before the later constant column,
	// then duplicates y after both have been assembled.
	count, err := e.set.Len()
	must(err)
	found := false
	for i := 0; i < count; i++ {
		rule, err := e.set.Rule(i)
		must(err)
		hd, err := rule.Head()
		must(err)
		d, err := m.GetDecl(hd)
		must(err)
		if d == u {
			_, plan, err := e.karrDatalog.compileUnaryRule(rule)
			must(err)
			if len(plan.headConstants) != 2 || plan.headConstants[0] != nil || plan.headConstants[1] == nil || plan.headConstants[1].Cmp(big.NewRat(4, 1)) != 0 || len(plan.duplicates) != 1 {
				t.Fatal("column assembly order", plan)
			}
			found = true
		}
		must(rule.Close())
	}
	if !found {
		t.Fatal("unary rule absent")
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
			t.Fatal("answer", values, want, expr)
		}
	}
	check(u, []string{"8", "1", "4", "8"}, true)
	check(u, []string{"9", "1", "4", "9"}, true)
	check(u, []string{"8", "1", "4", "7"}, false)
	check(u, []string{"8", "2", "4", "8"}, false)
	check(b, []string{"8", "3", "8", "1"}, true)
	check(b, []string{"8", "3", "7", "1"}, false)
	check(b, []string{"8", "2", "8", "1"}, false)
}

func TestKarrUnboundHeadCancellationAndOverflow(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(err)
	defer func() { must(e.Close()) }()
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
	p, q := decl("unbound-head-cancel-p", 1), decl("unbound-head-cancel-q", 3)
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	body, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	head, err := m.MkApp(q, []ast.ASTHandle{y, x, y})
	must(err)
	f, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(f, ""))
	must(e.flushPendingRules())
	a, err := newKarrDatalogActivation(m, e.set, []ast.ASTHandle{p, q})
	must(err)
	defer func() { must(a.close()) }()
	armed, fired := true, false
	seed := rel.NewKarrEmpty(1, func() bool {
		if armed {
			fired = true
			m.Limit.Cancel()
			return false
		}
		return true
	})
	must(seed.AddFact([]*big.Rat{big.NewRat(1, 1)}))
	a.seeds[p] = seed
	status, err := a.query([]ast.ASTHandle{q})
	m.Limit.ResetCancel()
	if !fired || err == nil || status != lbool.Undef || a.results != nil {
		t.Fatal("cancel", fired, status, err)
	}
	armed = false
	status, err = a.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef || a.results[q].Empty() {
		t.Fatal("retry", status)
	}
	oversized := rel.NewKarrEmpty(1, nil)
	large, ok := new(big.Rat).SetString("9223372036854775808")
	if !ok {
		t.Fatal("numeral")
	}
	must(oversized.AddFact([]*big.Rat{large}))
	a.seeds[p] = oversized
	status, err = a.query([]ast.ASTHandle{q})
	if status != lbool.Undef || !errors.Is(err, hilbert.ErrOverflow) || a.results != nil {
		t.Fatal("overflow", status, err)
	}
	a.seeds[p] = seed
	status, err = a.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef {
		t.Fatal("overflow retry", status)
	}
}
