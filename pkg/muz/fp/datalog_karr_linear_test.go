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

func TestKarrLinearExpressionRowsAndRejection(t *testing.T) {
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
	z, err := m.MkVar(2, integer)
	must(err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	must(err)
	app := func(op int32, args ...ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkFamilyApp(family, op, nil, args)
		must(err)
		return h
	}
	pos := map[ast.ASTHandle]uint32{x: 0, y: 1}
	a := &karrDatalogActivation{manager: m}
	for _, tc := range []struct {
		name    string
		expr    ast.ASTHandle
		x, y, c int64
	}{
		{"sum", app(6, x, y, three), 1, 1, 3}, {"subtract", app(7, x, y), 1, -1, 0}, {"minus", app(8, x), -1, 0, 0},
		{"mul-left", app(9, two, x), 2, 0, 0}, {"mul-right", app(9, y, two), 0, 2, 0}, {"nested", app(8, app(7, app(9, two, x), three)), -2, 0, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := make([]big.Rat, 2)
			var c big.Rat
			must(a.addKarrLinear(tc.expr, big.NewRat(1, 1), pos, row, &c, integer, family))
			if row[0].Cmp(big.NewRat(tc.x, 1)) != 0 || row[1].Cmp(big.NewRat(tc.y, 1)) != 0 || c.Cmp(big.NewRat(tc.c, 1)) != 0 {
				t.Fatal("row", row, c.String())
			}
		})
	}
	for _, expr := range []ast.ASTHandle{app(9, x, y), z} {
		row := make([]big.Rat, 2)
		var c big.Rat
		if err := a.addKarrLinear(expr, big.NewRat(1, 1), pos, row, &c, integer, family); !errors.Is(err, ErrKarrDatalogFragment) {
			t.Fatal("unsupported linear term", err)
		}
	}
}

func TestDatalogKarrLinearGuardPublicAndRetry(t *testing.T) {
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
		d, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := decl("karr-linear-p"), decl("karr-linear-q")
	must(e.AddTableFact(p, []uint32{2, 3}))
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	mul, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{two, x})
	must(err)
	sum, err := m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{mul, one})
	must(err)
	guard, err := m.MkFamilyApp(family, 3, nil, []ast.ASTHandle{sum, y})
	must(err)
	input, err := m.MkApp(p, []ast.ASTHandle{x, y})
	must(err)
	output, err := m.MkApp(q, []ast.ASTHandle{x, y})
	must(err)
	body, err := m.MkAnd(input, guard)
	must(err)
	formula, err := m.MkImplies(body, output)
	must(err)
	must(e.AddRule(formula, ""))
	status, err := e.QueryPredicate(q)
	must(err)
	if status != lbool.Undef {
		t.Fatal("query", status)
	}
	rule, err := e.set.Rule(0)
	must(err)
	_, plan, err := e.karrDatalog.compileGuardRule(rule)
	must(err)
	must(rule.Close())
	if len(plan.row) != 2 || plan.row[0].Cmp(big.NewRat(2, 1)) != 0 || plan.row[1].Cmp(big.NewRat(-1, 1)) != 0 || plan.constant.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("linear guard", plan)
	}
	filtered, err := e.karrDatalog.applyGuardRule(plan, rel.NewKarrFull(2, nil))
	must(err)
	rows, err := filtered.Inequalities()
	must(err)
	if rows.Len() != 1 {
		t.Fatal("linear filter rows", rows)
	}
	for _, tc := range []struct {
		x, y int64
		want bool
	}{{2, 3, true}, {1, 4, false}} {
		value := new(big.Rat).Mul(&rows.A[0][0], big.NewRat(tc.x, 1))
		value.Add(value, new(big.Rat).Mul(&rows.A[0][1], big.NewRat(tc.y, 1)))
		value.Add(value, &rows.B[0])
		if (value.Sign() >= 0) != tc.want {
			t.Fatal("linear valuation", tc, rows)
		}
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
		values []string
		want   bool
	}{{[]string{"2", "3"}, true}, {[]string{"2", "4"}, false}} {
		args := make([]ast.ASTHandle, 2)
		for i, v := range tc.values {
			args[i], err = m.ArithmeticIntegerNumeralText(v)
			must(err)
		}
		ref, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(subst.Substitute(expr, args, ref))
		h, err := m.RefGet(ref)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(e.guard.RewriteWithBindings(h, nil, reduced))
		h, err = m.RefGet(reduced)
		must(err)
		if (tc.want && !m.IsTrue(h)) || (!tc.want && !m.IsFalse(h)) {
			t.Fatal("answer", tc.values, tc.want, h)
		}
		must(m.DelRef(&ref))
		must(m.DelRef(&reduced))
	}
	armed, fired := false, false
	seed := rel.NewKarrEmpty(2, func() bool {
		if armed {
			fired = true
			m.Limit.Cancel()
			return false
		}
		return true
	})
	fact := rel.NewKarrEmpty(2, nil)
	must(fact.AddFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)}))
	_, err = seed.Union(fact)
	must(err)
	e.karrDatalog.seeds[p] = seed
	armed = true
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
	over := rel.NewKarrEmpty(2, nil)
	must(over.AddFact([]*big.Rat{large, big.NewRat(3, 1)}))
	e.karrDatalog.seeds[p] = over
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
