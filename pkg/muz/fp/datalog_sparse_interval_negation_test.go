package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogSparseIntervalExactFiniteNegationUsesLowerStrata(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	declare := func(text string, sorts []ast.ASTHandle, kinds []string) ast.ASTHandle {
		name, cause := m.Symbols.Intern(text)
		must(cause)
		declaration, cause := m.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		if kinds != nil {
			must(e.SetPredicateRepresentation(declaration, kinds))
		}
		return declaration
	}
	// Deliberately declare consumers before their negative producers. A
	// positive-only schedule would evaluate these rules too early.
	out := declare("heterogeneous-negation-out", []ast.ASTHandle{boolean}, nil)
	constantOut := declare("heterogeneous-negation-constant", []ast.ASTHandle{boolean}, nil)
	repeatedOut := declare("heterogeneous-negation-repeated", []ast.ASTHandle{boolean}, nil)
	unboundOut := declare("heterogeneous-negation-unbound", []ast.ASTHandle{boolean}, nil)
	complementOut := declare("heterogeneous-negation-complement", []ast.ASTHandle{boolean}, nil)
	cycleA := declare("heterogeneous-negation-cycle-a", []ast.ASTHandle{boolean}, nil)
	cycleB := declare("heterogeneous-negation-cycle-b", []ast.ASTHandle{boolean}, nil)
	negative := declare("heterogeneous-negation-negative", []ast.ASTHandle{boolean}, nil)
	negativePair := declare("heterogeneous-negation-pair", []ast.ASTHandle{boolean, boolean}, nil)
	negativeFullPair := declare("heterogeneous-negation-full-pair", []ast.ASTHandle{boolean, boolean}, nil)
	seed := declare("heterogeneous-negation-seed", []ast.ASTHandle{boolean}, nil)
	positive := declare("heterogeneous-negation-positive", []ast.ASTHandle{boolean}, nil)
	_ = declare("heterogeneous-negation-mixed-activation", []ast.ASTHandle{boolean, integer}, []string{"tr_sparse", "interval_relation"})
	must(e.AddTableFact(positive, []uint32{0}))
	must(e.AddTableFact(positive, []uint32{1}))
	must(e.AddTableFact(seed, []uint32{0}))
	must(e.AddTableFact(negativePair, []uint32{0, 0}))
	must(e.AddTableFact(negativeFullPair, []uint32{0, 0}))
	must(e.AddTableFact(negativeFullPair, []uint32{0, 1}))
	b, err := m.MkVar(0, boolean)
	must(err)
	c, err := m.MkVar(1, boolean)
	must(err)
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		result, cause := m.MkApp(declaration, arguments)
		must(cause)
		return result
	}
	addRule := func(body, head ast.ASTHandle, name string) {
		rule, cause := m.MkImplies(body, head)
		must(cause)
		must(e.addRuleEager(rule, name))
	}
	addRule(app(seed, b), app(negative, b), "heterogeneous-negation-lower-stratum")
	notNegative, err := m.MkNot(app(negative, b))
	must(err)
	body, err := m.MkAnd(app(positive, b), notNegative)
	must(err)
	addRule(body, app(out, b), "heterogeneous-negation-variable")
	falsity, err := m.MkFalse()
	must(err)
	notConstant, err := m.MkNot(app(negative, falsity))
	must(err)
	constantBody, err := m.MkAnd(app(positive, b), notConstant)
	must(err)
	addRule(constantBody, app(constantOut, b), "heterogeneous-negation-constant")
	notRepeated, err := m.MkNot(app(negativePair, b, b))
	must(err)
	repeatedBody, err := m.MkAnd(app(positive, b), notRepeated)
	must(err)
	addRule(repeatedBody, app(repeatedOut, b), "heterogeneous-negation-repeated")
	notUnbound, err := m.MkNot(app(negativeFullPair, b, c))
	must(err)
	unboundBody, err := m.MkAnd(app(positive, b), notUnbound)
	must(err)
	addRule(unboundBody, app(unboundOut, b), "heterogeneous-negation-unbound")
	notComplement, err := m.MkNot(app(negative, b))
	must(err)
	addRule(notComplement, app(complementOut, b), "heterogeneous-negation-complement")
	for _, tc := range []struct {
		declaration ast.ASTHandle
		status      lbool.LBool
		facts       []uint64
	}{
		{out, lbool.True, []uint64{1}},
		{constantOut, lbool.False, nil},
		{repeatedOut, lbool.True, []uint64{1}},
		{unboundOut, lbool.True, []uint64{1}},
		{complementOut, lbool.True, []uint64{1}},
	} {
		status, cause := e.QueryPredicate(tc.declaration)
		must(cause)
		if status != tc.status {
			t.Fatal("finite negation status", tc.declaration, status, tc.status)
		}
		snapshots, cause := e.DatalogRelationSnapshots([]ast.ASTHandle{tc.declaration})
		must(cause)
		if tc.facts == nil {
			if len(snapshots) != 1 || len(snapshots[0].Facts) != 0 {
				t.Fatal("constant negation should remove every row", snapshots)
			}
			continue
		}
		if len(snapshots) != 1 || len(snapshots[0].Facts) != 1 || len(snapshots[0].Facts[0]) != len(tc.facts) {
			t.Fatal("finite negation snapshot", snapshots)
		}
		for i, value := range tc.facts {
			if snapshots[0].Facts[0][i] != value {
				t.Fatal("finite negation retained wrong row", snapshots)
			}
		}
	}
	notCycleB, err := m.MkNot(app(cycleB, b))
	must(err)
	cycleABody, err := m.MkAnd(app(positive, b), notCycleB)
	must(err)
	addRule(cycleABody, app(cycleA, b), "heterogeneous-negation-cycle-a")
	notCycleA, err := m.MkNot(app(cycleA, b))
	must(err)
	cycleBBody, err := m.MkAnd(app(positive, b), notCycleA)
	must(err)
	addRule(cycleBBody, app(cycleB, b), "heterogeneous-negation-cycle-b")
	status, cause := e.QueryPredicate(cycleA)
	if status != lbool.Undef || !errors.Is(cause, ErrSparseIntervalDatalogFragment) {
		t.Fatal("negative dependency cycle was not rejected", status, cause)
	}
}

func TestDatalogSparseIntervalNegationRejectsMixedNegative(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	declare := func(text string, sorts []ast.ASTHandle, kinds []string) ast.ASTHandle {
		name, cause := m.Symbols.Intern(text)
		must(cause)
		declaration, cause := m.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		if kinds != nil {
			must(e.SetPredicateRepresentation(declaration, kinds))
		}
		return declaration
	}
	positive := declare("heterogeneous-mixed-negation-positive", []ast.ASTHandle{boolean}, nil)
	mixed := declare("heterogeneous-mixed-negation-negative", []ast.ASTHandle{boolean, integer}, []string{"tr_sparse", "interval_relation"})
	out := declare("heterogeneous-mixed-negation-out", []ast.ASTHandle{boolean}, nil)
	must(e.AddTableFact(positive, []uint32{1}))
	b, err := m.MkVar(0, boolean)
	must(err)
	x, err := m.MkVar(1, integer)
	must(err)
	positiveApp, err := m.MkApp(positive, []ast.ASTHandle{b})
	must(err)
	mixedApp, err := m.MkApp(mixed, []ast.ASTHandle{b, x})
	must(err)
	notMixed, err := m.MkNot(mixedApp)
	must(err)
	body, err := m.MkAnd(positiveApp, notMixed)
	must(err)
	head, err := m.MkApp(out, []ast.ASTHandle{b})
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(rule, "heterogeneous-mixed-negation-refusal"))
	status, cause := e.QueryPredicate(out)
	if status != lbool.Undef || !errors.Is(cause, ErrSparseIntervalDatalogFragment) {
		t.Fatal("mixed negative relation was silently accepted", status, cause)
	}
}
