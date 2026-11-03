package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source: rel_context.cpp::transform_rules runs the interpreted-tail
// simplifier at priority 40000, before filter rules and simple joins.
func TestDatalogInterpretedEqualityPrecedesFiniteCompilation(t *testing.T) {
	for _, withServices := range []bool{true, false} {
		name := "services"
		if !withServices {
			name = "without-services"
		}
		t.Run(name, func(t *testing.T) { testDatalogInterpretedEqualityPrecedesFiniteCompilation(t, withServices) })
	}
}

func testDatalogInterpretedEqualityPrecedesFiniteCompilation(t *testing.T, withServices bool) {
	t.Helper()
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("interpreted-tail query leaked bytes", budget.Used())
		}
	})
	m, globals := newPDRLifecycleServices(t)
	budget = m.Budget()
	var e *PropositionalEngine
	var err error
	if withServices {
		e, err = NewPropositionalEngineWithServices(m, globals.Context, globals)
	} else {
		e, err = NewPropositionalEngine(m)
	}
	must(err)
	t.Cleanup(func() { must(e.Close()) })
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string, arity int) ast.ASTHandle {
		symbol, err := m.Symbols.Intern(name)
		must(err)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = boolean
		}
		declaration, err := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	predicate := declare("interp-tail-source", 2)
	output := declare("interp-tail-output", 1)
	truth, err := m.MkTrue()
	must(err)
	falsity, err := m.MkFalse()
	must(err)
	must(e.AddFact(predicate, []ast.ASTHandle{falsity, truth}))
	must(e.AddFact(predicate, []ast.ASTHandle{truth, falsity}))
	x, err := m.MkVar(1, boolean)
	must(err)
	y, err := m.MkVar(0, boolean)
	must(err)
	tail, err := m.MkApp(predicate, []ast.ASTHandle{x, y})
	must(err)
	guard, err := m.MkEq(y, truth)
	must(err)
	body, err := m.MkAnd(tail, guard)
	must(err)
	head, err := m.MkApp(output, []ast.ASTHandle{x})
	must(err)
	formula, err := m.MkImplies(body, head)
	must(err)
	must(e.AddRule(formula, "interp-tail-rule"))
	original, err := e.set.Rule(0)
	must(err)
	defer func() { must(original.Close()) }()
	originalPredicates := len(e.predicates)
	for query := 0; query < 3; query++ {
		if query == 2 {
			must(e.AddFact(predicate, []ast.ASTHandle{truth, truth}))
		}
		status, err := e.QueryPredicate(output)
		must(err)
		if status != lbool.True || !e.compiled {
			t.Fatal("guarded finite query", query, status, e.compiled)
		}
		facts, err := e.predicates[output].table.Facts()
		must(err)
		wantCount := 1
		if query == 2 {
			wantCount = 2
		}
		seen := [2]bool{}
		for _, fact := range facts {
			if len(fact) != 1 || fact[0] > 1 {
				t.Fatal("invalid Boolean result", facts)
			}
			seen[fact[0]] = true
		}
		if len(facts) != wantCount || !seen[0] || seen[1] != (query == 2) {
			t.Fatal("equality propagation changed finite rows", query, facts)
		}
		for i := 0; i < e.datalogStats.Size(); i++ {
			key := e.datalogStats.Key(i)
			if key == "dl.filter" || key == "dl.filter_interpreted_project" {
				t.Fatal("interpreted equality survived preprocessing", key, e.datalogStats.UintValue(i))
			}
		}
		current, err := e.set.Rule(0)
		must(err)
		same, err := current.Equal(original)
		must(err)
		must(current.Close())
		if !same || len(e.predicates) != originalPredicates {
			t.Fatal("query normalization changed persistent program", query)
		}
		if e.datalogTransformCleanup != nil || (!withServices && (e.params != nil || e.globals != nil)) {
			t.Fatal("query-local services escaped preprocessing", query)
		}
	}
}
