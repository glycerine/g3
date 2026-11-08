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

func TestKarrDisjunctionRetainedRuleShape(t *testing.T) {
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
	decl := func(name string) ast.ASTHandle {
		sym, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := decl("karr-disj-shape-p"), decl("karr-disj-shape-q")
	x, err := m.MkVar(0, integer)
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	must(err)
	eq1, err := m.MkEq(x, one)
	must(err)
	eq2, err := m.MkEq(x, three)
	must(err)
	guard, err := m.MkOr(eq1, eq2)
	must(err)
	input, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	head, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	body, err := m.MkAnd(input, guard)
	must(err)
	formula, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(formula, ""))
	must(e.flushPendingRules())
	n, err := e.set.Len()
	must(err)
	if n != 1 {
		t.Fatalf("OR split into %d rules", n)
	}
	rule, err := e.set.Rule(0)
	must(err)
	defer func() { must(rule.Close()) }()
	n, err = rule.TailSize()
	must(err)
	u, err := rule.UninterpretedTailSize()
	must(err)
	if n != 2 || u != 1 {
		t.Fatalf("rule shape %d/%d", n, u)
	}
	tail, neg, err := rule.Tail(1)
	must(err)
	op, err := m.GetDeclKind(tail)
	must(err)
	if neg || op != ast.ASTOpOr {
		t.Fatalf("interpreted disjunction %v %d", neg, op)
	}
}

func TestDatalogKarrDisjunctionPublicAndCancellationRetry(t *testing.T) {
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
		d, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := decl("karr-guard-p"), decl("karr-guard-q")
	must(e.AddTableFact(p, []uint32{2}))
	x, err := m.MkVar(0, integer)
	must(err)
	input, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	output, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	must(err)
	eq1, err := m.MkEq(three, x)
	must(err)
	eq2, err := m.MkEq(x, one)
	must(err)
	guard, err := m.MkOr(eq1, eq2)
	must(err)
	body, err := m.MkAnd(input, guard)
	must(err)
	f, err := m.MkImplies(body, output)
	must(err)
	must(e.addRuleEager(f, ""))
	status, err := e.QueryPredicate(q)
	must(err)
	if status != lbool.Undef {
		t.Fatal("guard query", status)
	}
	result := e.karrDatalog.results[q]
	rows, err := result.Inequalities()
	must(err)
	if rows.Len() == 0 {
		t.Fatal("lost input constraint", rows)
	}
	answer, err := e.Answer(q)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	expr, err := m.RefGet(answer)
	must(err)
	subst, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(subst.Close()) }()
	for _, tc := range []struct {
		value string
		want  bool
	}{{"2", true}, {"1", false}} {
		value, err := m.ArithmeticIntegerNumeralText(tc.value)
		must(err)
		ref, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(subst.Substitute(expr, []ast.ASTHandle{value}, ref))
		h, err := m.RefGet(ref)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(e.guard.RewriteWithBindings(h, nil, reduced))
		h, err = m.RefGet(reduced)
		must(err)
		if (tc.want && !m.IsTrue(h)) || (!tc.want && !m.IsFalse(h)) {
			t.Fatal("answer", tc.value, tc.want, h)
		}
		must(m.DelRef(&ref))
		must(m.DelRef(&reduced))
	}
	r, err := e.set.Rule(0)
	must(err)
	_, plan, err := e.karrDatalog.compileGuardRule(r)
	must(err)
	must(r.Close())
	if len(plan.row) != 1 || plan.row[0].Cmp(big.NewRat(1, 1)) != 0 || plan.constant.Cmp(big.NewRat(-1, 1)) != 0 || len(plan.secondRow) != 1 || plan.secondRow[0].Cmp(big.NewRat(-1, 1)) != 0 || plan.secondConstant.Cmp(big.NewRat(3, 1)) != 0 || plan.equal {
		t.Fatal("guard plan", plan)
	}
	// Observe the native filter before union dualizes its inequalities.
	// The retained Karr union is an abstraction, so inspect the actual
	// interval rows here rather than infer them from a singleton input.
	filtered, err := e.karrDatalog.applyGuardRule(plan, rel.NewKarrFull(1, nil))
	must(err)
	interval, err := filtered.Inequalities()
	must(err)
	if interval.Len() != 2 {
		t.Fatal("interval rows", interval)
	}
	for _, tc := range []struct {
		value int64
		want  bool
	}{{0, false}, {1, true}, {2, true}, {3, true}, {4, false}} {
		accepted := true
		for i := range interval.A {
			v := new(big.Rat).Mul(&interval.A[i][0], big.NewRat(tc.value, 1))
			v.Add(v, &interval.B[i])
			if v.Sign() < 0 {
				accepted = false
			}
		}
		if accepted != tc.want {
			t.Fatal("interval", tc.value, tc.want, interval)
		}
	}
	// An input whose cached inequalities must be reconstructed is interrupted
	// during Hilbert dualization, and the same activation can then be retried.
	armed, fired := true, false
	source := rel.NewKarrEmpty(1, func() bool {
		if armed {
			fired = true
			m.Limit.Cancel()
			return false
		}
		return true
	})
	must(source.AddFact([]*big.Rat{big.NewRat(2, 1)}))
	seed := rel.NewKarrEmpty(1, func() bool {
		if armed {
			fired = true
			m.Limit.Cancel()
			return false
		}
		return true
	})
	armed = false
	_, err = seed.Union(source)
	must(err)
	armed = true
	fired = false
	e.karrDatalog.seeds[p] = seed
	status, err = e.karrDatalog.query([]ast.ASTHandle{q})
	m.Limit.ResetCancel()
	if !fired || err == nil || status != lbool.Undef || e.karrDatalog.results != nil {
		t.Fatal("cancel", fired, status, err)
	}
	armed = false
	status, err = e.karrDatalog.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef {
		t.Fatal("retry", status)
	}
	large, ok := new(big.Rat).SetString("9223372036854775808")
	if !ok {
		t.Fatal("numeral")
	}
	oversized := rel.NewKarrEmpty(1, nil)
	must(oversized.AddFact([]*big.Rat{large}))
	e.karrDatalog.seeds[p] = oversized
	status, err = e.karrDatalog.query([]ast.ASTHandle{q})
	if status != lbool.Undef || !errors.Is(err, hilbert.ErrOverflow) || e.karrDatalog.results != nil {
		t.Fatal("overflow", status, err)
	}
	e.karrDatalog.seeds[p] = seed
	status, err = e.karrDatalog.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef {
		t.Fatal("overflow retry", status)
	}
}

func TestKarrDisjunctionBoundsAndUnsupportedShapes(t *testing.T) {
	m, _ := newPDRLifecycleServices(t)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	must(err)
	eq := func(a, b ast.ASTHandle) ast.ASTHandle { h, err := m.MkEq(a, b); must(err); return h }
	makeOr := func(a, b ast.ASTHandle) ast.ASTHandle { h, err := m.MkOr(a, b); must(err); return h }
	a := &karrDatalogActivation{manager: m}
	positions := map[ast.ASTHandle]uint32{x: 0, y: 1}
	for _, guard := range []ast.ASTHandle{makeOr(eq(x, three), eq(one, x)), makeOr(eq(one, x), eq(x, three))} {
		var p karrGuardRule
		must(a.compileKarrDisjunction(guard, positions, 2, &p))
		if p.row[0].Cmp(big.NewRat(1, 1)) != 0 || p.row[1].Sign() != 0 || p.constant.Cmp(big.NewRat(-1, 1)) != 0 || p.secondRow[0].Cmp(big.NewRat(-1, 1)) != 0 || p.secondConstant.Cmp(big.NewRat(3, 1)) != 0 {
			t.Fatal("bounds", p)
		}
	}
	var duplicate karrGuardRule
	must(a.compileKarrDisjunction(makeOr(eq(x, one), eq(one, x)), positions, 2, &duplicate))
	if duplicate.constant.Cmp(big.NewRat(-1, 1)) != 0 || duplicate.secondConstant.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("duplicate bound", duplicate)
	}
	for _, guard := range []ast.ASTHandle{makeOr(eq(x, one), eq(y, three)), makeOr(eq(x, one), eq(x, y))} {
		var p karrGuardRule
		if err := a.compileKarrDisjunction(guard, positions, 2, &p); !errors.Is(err, ErrKarrDatalogFragment) {
			t.Fatal("unsupported OR", err)
		}
	}
}
