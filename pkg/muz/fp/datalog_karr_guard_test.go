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

func TestDatalogKarrGuardPublicAndCancellationRetry(t *testing.T) {
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
	must(e.AddTableFact(p, []uint32{1}))
	x, err := m.MkVar(0, integer)
	must(err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	input, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	output, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	guard, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{x, two})
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
	}{{"1", true}, {"2", false}} {
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
	if len(plan.row) != 1 || plan.row[0].Cmp(big.NewRat(-1, 1)) != 0 || plan.constant.Cmp(big.NewRat(2, 1)) != 0 || plan.equal {
		t.Fatal("guard plan", plan)
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
	must(source.AddFact([]*big.Rat{big.NewRat(1, 1)}))
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

func TestDatalogKarrNegatedGuardPublicAndCancellationRetry(t *testing.T) {
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
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	input, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	output, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	guard, err := m.MkFamilyApp(family, 4, nil, []ast.ASTHandle{x, two})
	must(err)
	guard, err = m.MkNot(guard)
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
	if len(plan.row) != 1 || plan.row[0].Cmp(big.NewRat(1, 1)) != 0 || plan.constant.Cmp(big.NewRat(-2, 1)) != 0 || plan.equal {
		t.Fatal("guard plan", plan)
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

func TestDatalogKarrGuardComparisonPlans(t *testing.T) {
	tests := []struct {
		name          string
		op            int32
		swap          bool
		equal         bool
		row, constant int64
	}{
		{"le", 2, false, false, -1, 2}, {"ge", 3, false, false, 1, -2}, {"lt", 4, false, false, -1, 1}, {"gt", 5, false, false, 1, -3},
		{"le-swapped", 2, true, false, 1, -2}, {"gt-swapped", 5, true, false, -1, 1},
		{"eq", -1, false, true, 1, -2}, {"eq-swapped", -1, true, true, -1, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
			p, q := decl("guard-table-p"), decl("guard-table-q")
			x, err := m.MkVar(0, integer)
			must(err)
			two, err := m.ArithmeticIntegerNumeralText("2")
			must(err)
			left, right := x, two
			if tc.swap {
				left, right = right, left
			}
			var guard ast.ASTHandle
			if tc.op == -1 {
				guard, err = m.MkEq(left, right)
			} else {
				guard, err = m.MkFamilyApp(family, tc.op, nil, []ast.ASTHandle{left, right})
			}
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
			a, err := newKarrDatalogActivation(m, e.set, []ast.ASTHandle{p, q})
			must(err)
			defer func() { must(a.close()) }()
			rule, err := e.set.Rule(0)
			must(err)
			_, plan, err := a.compileGuardRule(rule)
			must(err)
			must(rule.Close())
			if plan.equal != tc.equal || plan.row[0].Cmp(big.NewRat(tc.row, 1)) != 0 || plan.constant.Cmp(big.NewRat(tc.constant, 1)) != 0 {
				t.Fatalf("plan %+v", plan)
			}
		})
	}
}

func TestDatalogKarrNegatedComparisonPlans(t *testing.T) {
	tests := []struct {
		name          string
		op            int32
		swap          bool
		equal         bool
		row, constant int64
	}{
		{"not-le", 2, false, false, 1, -3}, {"not-ge", 3, false, false, -1, 1},
		{"not-lt", 4, false, false, 1, -2}, {"not-gt", 5, false, false, -1, 2},
		{"not-le-swapped", 2, true, false, -1, 1}, {"not-gt-swapped", 5, true, false, 1, -2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
			p, q := decl("guard-table-p"), decl("guard-table-q")
			x, err := m.MkVar(0, integer)
			must(err)
			two, err := m.ArithmeticIntegerNumeralText("2")
			must(err)
			left, right := x, two
			if tc.swap {
				left, right = right, left
			}
			var guard ast.ASTHandle
			if tc.op == -1 {
				guard, err = m.MkEq(left, right)
			} else {
				guard, err = m.MkFamilyApp(family, tc.op, nil, []ast.ASTHandle{left, right})
			}
			must(err)
			guard, err = m.MkNot(guard)
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
			a, err := newKarrDatalogActivation(m, e.set, []ast.ASTHandle{p, q})
			must(err)
			defer func() { must(a.close()) }()
			rule, err := e.set.Rule(0)
			must(err)
			_, plan, err := a.compileGuardRule(rule)
			must(err)
			must(rule.Close())
			if plan.equal != tc.equal || plan.row[0].Cmp(big.NewRat(tc.row, 1)) != 0 || plan.constant.Cmp(big.NewRat(tc.constant, 1)) != 0 {
				t.Fatalf("plan %+v", plan)
			}
		})
	}
}

func TestDatalogKarrGuardCachedBasisAndExpressionClone(t *testing.T) {
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
	p, q := decl("guard-cached-p"), decl("guard-cached-q")
	x, err := m.MkVar(0, integer)
	must(err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	input, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	output, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	fact, err := m.MkApp(p, []ast.ASTHandle{one})
	must(err)
	must(e.addRuleEager(fact, ""))
	guard, err := m.MkFamilyApp(family, 4, nil, []ast.ASTHandle{x, zero})
	must(err)
	body, err := m.MkAnd(input, guard)
	must(err)
	formula, err := m.MkImplies(body, output)
	must(err)
	must(e.addRuleEager(formula, ""))
	status, err := e.QueryPredicate(q)
	must(err)
	if status != lbool.Undef {
		t.Fatal("status", status)
	}
	answer, err := e.Answer(q)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	h, err := m.RefGet(answer)
	must(err)
	subst, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(subst.Close()) }()
	grounded, err := m.NewRef(ast.ASTHandle{})
	must(err)
	defer func() { must(m.DelRef(&grounded)) }()
	must(subst.Substitute(h, []ast.ASTHandle{one}, grounded))
	h, err = m.RefGet(grounded)
	must(err)
	reduced, err := m.NewRef(ast.ASTHandle{})
	must(err)
	defer func() { must(m.DelRef(&reduced)) }()
	must(e.guard.RewriteWithBindings(h, nil, reduced))
	h, err = m.RefGet(reduced)
	must(err)
	if !m.IsTrue(h) {
		t.Fatal("native cached basis should retain contradictory fact", h)
	}
	status, err = e.QueryExpression(output)
	must(err)
	if status != lbool.Undef || e.queryClone == nil || e.queryClone.karrDatalog == e.karrDatalog {
		t.Fatal("clone", status)
	}
	held, err := e.ExpressionAnswer()
	must(err)
	must(e.clearQueryResources())
	if _, err = m.RefGet(held); err != nil {
		t.Fatal("answer lifetime", err)
	}
	must(m.DelRef(&held))
}

func TestDatalogKarrGuardTwoVariablesAndHeadPermutation(t *testing.T) {
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
		d, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := decl("guard-vars-p"), decl("guard-vars-q")
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	input, err := m.MkApp(p, []ast.ASTHandle{x, y})
	must(err)
	head, err := m.MkApp(q, []ast.ASTHandle{y, x})
	must(err)
	guard, err := m.MkFamilyApp(family, 5, nil, []ast.ASTHandle{y, x})
	must(err)
	body, err := m.MkAnd(input, guard)
	must(err)
	f, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(f, ""))
	must(e.flushPendingRules())
	a, err := newKarrDatalogActivation(m, e.set, []ast.ASTHandle{p, q})
	must(err)
	defer func() { must(a.close()) }()
	rule, err := e.set.Rule(0)
	must(err)
	_, plan, err := a.compileGuardRule(rule)
	must(err)
	must(rule.Close())
	if len(plan.row) != 2 || plan.row[0].Cmp(big.NewRat(-1, 1)) != 0 || plan.row[1].Cmp(big.NewRat(1, 1)) != 0 || plan.constant.Cmp(big.NewRat(-1, 1)) != 0 || len(plan.head.cycles) != 1 {
		t.Fatal("two-variable guard plan", plan)
	}
}
