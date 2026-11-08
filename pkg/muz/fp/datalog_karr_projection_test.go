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

func TestDatalogKarrProjectionPermutationPublic(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
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
		s, err := m.Symbols.Intern(name)
		must(err)
		sorts := make([]ast.ASTHandle, n)
		for i := range sorts {
			sorts[i] = integer
		}
		d, err := m.MkFuncDecl(s, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, rotated, projected, unit := decl("karr-projection-p", 3), decl("karr-projection-rotate", 3), decl("karr-projection-pair", 2), decl("karr-projection-unit", 0)
	assembled := decl("karr-projection-assembled", 5)
	repeatedBody := decl("karr-projection-repeated-body", 2)
	repeatedHead := decl("karr-projection-repeated-head", 3)
	must(e.AddTableFact(p, []uint32{1, 2, 3}))
	must(e.AddTableFact(repeatedBody, []uint32{4, 4}))
	vars := make([]ast.ASTHandle, 3)
	for i := range vars {
		vars[i], err = m.MkVar(uint32(i), integer)
		must(err)
	}
	body, err := m.MkApp(p, vars)
	must(err)
	addRule := func(target ast.ASTHandle, args []ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(target, args)
		must(err)
		f, err := m.MkImplies(body, h)
		must(err)
		must(e.addRuleEager(f, ""))
		return h
	}
	addRule(rotated, []ast.ASTHandle{vars[2], vars[0], vars[1]})
	query := addRule(projected, []ast.ASTHandle{vars[2], vars[0]})
	addRule(unit, nil)
	seven, err := m.ArithmeticIntegerNumeralText("7")
	must(err)
	assembledQuery := addRule(assembled, []ast.ASTHandle{vars[2], seven, vars[0], vars[2], seven})
	repeatedTail, err := m.MkApp(repeatedBody, []ast.ASTHandle{vars[0], vars[0]})
	must(err)
	repeatedResult, err := m.MkApp(repeatedHead, []ast.ASTHandle{vars[0], vars[0], vars[0]})
	must(err)
	repeatedRule, err := m.MkImplies(repeatedTail, repeatedResult)
	must(err)
	must(e.addRuleEager(repeatedRule, ""))
	substitution, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(substitution.Close()) }()
	check := func(d ast.ASTHandle, values []string, want bool) {
		status, err := e.QueryPredicate(d)
		must(err)
		if status != lbool.Undef {
			t.Fatal("imprecise result", status)
		}
		answer, err := e.Answer(d)
		must(err)
		defer func() { must(m.DelRef(&answer)) }()
		h, err := m.RefGet(answer)
		must(err)
		args := make([]ast.ASTHandle, len(values))
		for i, value := range values {
			args[i], err = m.ArithmeticIntegerNumeralText(value)
			must(err)
		}
		ground, err := m.NewRef(ast.ASTHandle{})
		must(err)
		defer func() { must(m.DelRef(&ground)) }()
		must(substitution.Substitute(h, args, ground))
		h, err = m.RefGet(ground)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		defer func() { must(m.DelRef(&reduced)) }()
		must(e.guard.RewriteWithBindings(h, nil, reduced))
		h, err = m.RefGet(reduced)
		must(err)
		if m.IsTrue(h) != want || (!m.IsTrue(h) && !m.IsFalse(h)) {
			t.Fatal("answer valuation", values, want, h)
		}
	}
	check(rotated, []string{"3", "1", "2"}, true)
	check(rotated, []string{"2", "3", "1"}, false)
	check(projected, []string{"3", "1"}, true)
	check(projected, []string{"1", "3"}, false)
	check(unit, nil, true)
	check(assembled, []string{"3", "7", "1", "3", "7"}, true)
	check(assembled, []string{"3", "7", "1", "2", "7"}, false)
	check(assembled, []string{"3", "8", "1", "3", "7"}, false)
	check(assembled, []string{"3", "7", "1", "3", "8"}, false)
	check(repeatedHead, []string{"4", "4", "4"}, true)
	check(repeatedHead, []string{"4", "4", "5"}, false)
	status, err := e.QueryExpression(assembledQuery)
	must(err)
	if status != lbool.Undef || e.queryClone == nil || e.queryClone.karrDatalog == e.karrDatalog {
		t.Fatal("assembled query clone", status)
	}
	status, err = e.QueryExpression(query)
	must(err)
	if status != lbool.Undef || e.queryClone == nil || e.queryClone.karrDatalog == e.karrDatalog {
		t.Fatal("query clone", status)
	}
	held, err := e.ExpressionAnswer()
	must(err)
	must(e.clearQueryResources())
	if _, err := m.RefGet(held); err != nil {
		t.Fatal("answer lifetime", err)
	}
	must(m.DelRef(&held))
	m.Limit.Cancel()
	status, err = e.QueryPredicate(projected)
	m.Limit.ResetCancel()
	if err == nil || status != lbool.Undef {
		t.Fatal("cancel", status, err)
	}
	check(projected, []string{"3", "1"}, true)
}

func TestDatalogKarrProjectionRepeatedHead(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
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
	declare := func(name string, n int) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		must(err)
		sorts := make([]ast.ASTHandle, n)
		for i := range sorts {
			sorts[i] = integer
		}
		d, err := m.MkFuncDecl(s, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := declare("karr-duplicate-p", 1), declare("karr-duplicate-q", 2)
	must(e.AddTableFact(p, []uint32{1}))
	x, err := m.MkVar(0, integer)
	must(err)
	body, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	head, err := m.MkApp(q, []ast.ASTHandle{x, x})
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(rule, ""))
	for i := 0; i < 2; i++ {
		status, err := e.QueryPredicate(q)
		if status != lbool.Undef || err != nil || e.karrDatalog.results == nil {
			t.Fatal("repeated head query", status, err)
		}
	}
}

func TestKarrDatalogProjectionCancellationDuringDualization(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string, n int) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		must(err)
		sorts := make([]ast.ASTHandle, n)
		for i := range sorts {
			sorts[i] = integer
		}
		d, err := m.MkFuncDecl(s, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := declare("karr-project-cancel-p", 2), declare("karr-project-cancel-q", 1)
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	body, err := m.MkApp(p, []ast.ASTHandle{x, y})
	must(err)
	head, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(rule, ""))
	must(e.flushPendingRules())
	activation, err := newKarrDatalogActivation(m, e.set, []ast.ASTHandle{p, q})
	must(err)
	defer func() { must(activation.close()) }()
	fired, armed := false, true
	seed := rel.NewKarrEmpty(2, func() bool {
		if armed {
			fired = true
			m.Limit.Cancel()
			return false
		}
		return true
	})
	must(seed.AddFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1)}))
	activation.seeds[p] = seed
	status, err := activation.query([]ast.ASTHandle{q})
	m.Limit.ResetCancel()
	if !fired || err == nil || status != lbool.Undef || activation.results != nil {
		t.Fatal("projection cancellation published result", fired, status, err)
	}
	armed = false
	status, err = activation.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef || activation.results[q].Empty() {
		t.Fatal("retry", status)
	}
	matrix, err := activation.results[q].Inequalities()
	must(err)
	if matrix.Len() == 0 {
		t.Fatal("retry lost projected constraint")
	}
	for i, row := range matrix.A {
		atOne := new(big.Rat).Add(&row[0], &matrix.B[i])
		if matrix.Eq[i] && atOne.Sign() != 0 || !matrix.Eq[i] && atOne.Sign() < 0 {
			t.Fatal("retry changed projected fact", matrix)
		}
	}
	// Checked Hilbert arithmetic must reject a projected oversized fact
	// without publishing the partially constructed snapshot.
	large := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 63))
	oversized := rel.NewKarrEmpty(2, nil)
	must(oversized.AddFact([]*big.Rat{large, big.NewRat(2, 1)}))
	activation.seeds[p] = oversized
	status, err = activation.query([]ast.ASTHandle{q})
	if status != lbool.Undef || !errors.Is(err, hilbert.ErrOverflow) || activation.results != nil {
		t.Fatal("projection overflow published result", status, err)
	}
	activation.seeds[p] = seed
	status, err = activation.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef || activation.results[q].Empty() {
		t.Fatal("overflow retry", status)
	}

}

func TestKarrRepeatedHeadDuplicationFailureRetry(t *testing.T) {
	source := rel.NewKarrEmpty(2, nil)
	fragmentMust(t, source.AddFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)}))
	rule := karrHeadPlan{duplicates: []uint32{1}}
	result, err := rule.apply(source)
	fragmentMust(t, err)
	if result.Dimension() != 3 || source.Dimension() != 2 || source.Empty() {
		t.Fatal("retry changed source or result", result.Dimension(), source.Dimension())
	}
	matrix, err := result.Inequalities()
	fragmentMust(t, err)
	for i, row := range matrix.A {
		value := new(big.Rat).Set(&matrix.B[i])
		for j, x := range []int64{2, 3, 3} {
			value.Add(value, new(big.Rat).Mul(&row[j], big.NewRat(x, 1)))
		}
		if matrix.Eq[i] && value.Sign() != 0 || !matrix.Eq[i] && value.Sign() < 0 {
			t.Fatal("retry lost fact", matrix)
		}
	}
	large := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 63))
	oversized := rel.NewKarrEmpty(2, nil)
	fragmentMust(t, oversized.AddFact([]*big.Rat{large, big.NewRat(3, 1)}))
	if result, err := rule.apply(oversized); result != nil || !errors.Is(err, hilbert.ErrOverflow) {
		t.Fatal("duplication overflow", result, err)
	}
	result, err = rule.apply(source)
	fragmentMust(t, err)
	if result.Empty() {
		t.Fatal("retry after overflow")
	}
}
