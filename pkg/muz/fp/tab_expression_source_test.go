package fp_test

// Source: src/muz/tab/tab_context.cpp imp::query initializes the false-head
// goal from the query rule without adding that rule to the program index.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func newTabExpressionSourceFixture(t *testing.T) *queryExpressionFixture {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	queryMust(t, err)
	m, err := context.Manager()
	queryMust(t, err)
	globals, err := context.GlobalParameters()
	queryMust(t, err)
	e, err := fp.NewPropositionalEngineWithServices(m, globals.Context, globals)
	queryMust(t, err)
	f := &queryExpressionFixture{t: t, context: context, manager: m, engine: e}
	t.Cleanup(func() {
		queryMust(t, e.Close())
		for i := len(f.refs) - 1; i >= 0; i-- {
			queryMust(t, m.DelRef(&f.refs[i]))
		}
		queryMust(t, context.Close())
	})
	queryMust(t, e.SelectEngine("tab"))
	return f
}

func tabExpressionSourceQuery(t *testing.T, f *queryExpressionFixture, query ast.ASTHandle, want lbool.LBool, unfolds uint32) ast.ASTRef {
	t.Helper()
	queryMust(t, f.engine.ResetStatistics())
	status, err := f.engine.QueryExpression(query)
	queryMust(t, err)
	if status != want {
		t.Fatalf("expression query result %v, want %v", status, want)
	}
	var stats statistics.Statistics
	queryMust(t, f.engine.CollectStatistics(&stats))
	var actual uint32
	for i := 0; i < stats.Size(); i++ {
		if stats.Key(i) == "tab.num_unfold" {
			if !stats.IsUint(i) {
				t.Fatal("noninteger TAB unfold count")
			}
			actual += stats.UintValue(i)
		}
	}
	if actual != unfolds {
		t.Fatalf("expression query unfolded %d rules, want exactly %d source program rules", actual, unfolds)
	}
	answer, err := f.engine.ExpressionAnswer()
	queryMust(t, err)
	first, err := f.engine.ExpressionModel()
	queryMust(t, err)
	defer func() { queryMust(t, first.Close()) }()
	second, err := f.engine.ExpressionModel()
	queryMust(t, err)
	defer func() { queryMust(t, second.Close()) }()
	if first == second || first.Core() == second.Core() {
		t.Fatal("expression model calls shared a caller owner")
	}
	for _, core := range []*model.ModelCore{first.Core(), second.Core()} {
		count, err := core.NumDeclarations()
		queryMust(t, err)
		if count != 0 {
			t.Fatal("TAB inherited model contains interpretations", count)
		}
	}
	return answer
}

// Compare asserted premises individually. Asserting both the reachable
// program and the negated query into a solver would make every leaf follow
// vacuously, including an erroneous synthetic query bridge.
func checkTabExpressionSourceProof(t *testing.T, f *queryExpressionFixture, answer ast.ASTRef, facts []ast.ASTHandle, queryBody, boundSort ast.ASTHandle) {
	t.Helper()
	m := f.manager
	proof, err := m.RefGet(answer)
	queryMust(t, err)
	conclusion, err := m.GetProofFact(proof)
	queryMust(t, err)
	if !m.IsFalse(conclusion) {
		t.Fatal("TAB proof does not conclude false")
	}
	checker, err := proofs.NewChecker(m)
	queryMust(t, err)
	valid, conditions, err := checker.Check(proof)
	queryMust(t, err)
	defer func() { queryMust(t, checker.ReleaseSideConditions(conditions)) }()
	if !valid {
		t.Fatal("TAB proof failed structural validation")
	}
	solver, err := f.context.NewSimpleSolver()
	queryMust(t, err)
	defer func() { queryMust(t, solver.Close()) }()
	for _, condition := range conditions {
		queryMust(t, solver.Push())
		queryMust(t, solver.Assert(f.pin(m.MkNot(condition))))
		status, err := solver.CheckSat(nil)
		queryMust(t, err)
		if status != lbool.False {
			t.Fatal("TAB proof side condition is not independently valid", status)
		}
		queryMust(t, solver.Pop(1))
	}
	queryTerms, err := m.FlattenAnd([]ast.ASTHandle{queryBody})
	queryMust(t, err)
	wantFacts := make(map[ast.ASTHandle]bool)
	for _, fact := range facts {
		wantFacts[fact] = false
	}
	seen := make(map[ast.ASTHandle]bool)
	todo := []ast.ASTHandle{proof}
	queryPremises, resolutions := 0, 0
	for len(todo) != 0 {
		node := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[node] {
			continue
		}
		seen[node] = true
		kind, err := m.GetDeclKind(node)
		queryMust(t, err)
		if kind == ast.ASTPrHyperResolve {
			resolutions++
		}
		if kind == ast.ASTPrAsserted {
			fact, err := m.GetProofFact(node)
			queryMust(t, err)
			if _, found := wantFacts[fact]; found {
				wantFacts[fact] = true
			} else {
				if boundSort != (ast.ASTHandle{}) {
					forall, err := m.IsForall(fact)
					queryMust(t, err)
					if !forall {
						t.Fatal("query premise is not universally closed")
					}
					count, err := m.GetNumDeclarations(fact)
					queryMust(t, err)
					if count != 1 {
						t.Fatal("query premise binder count", count)
					}
					sort, err := m.GetDeclarationSort(fact, 0)
					queryMust(t, err)
					if sort != boundSort {
						t.Fatal("query premise binder sort")
					}
					fact, err = m.GetQuantifierBody(fact)
					queryMust(t, err)
				}
				kind, err := m.GetDeclKind(fact)
				queryMust(t, err)
				if kind != ast.ASTOpImplies {
					t.Fatal("unexpected asserted query premise")
				}
				body, err := m.GetArg(fact, 0)
				queryMust(t, err)
				head, err := m.GetArg(fact, 1)
				queryMust(t, err)
				if !m.IsFalse(head) {
					t.Fatal("asserted query premise has a synthetic predicate head")
				}
				// Rule normalization may reverse conjunction order. Require
				// precisely the original atoms, with no synthetic predicate.
				terms, err := m.FlattenAnd([]ast.ASTHandle{body})
				queryMust(t, err)
				counts := make(map[ast.ASTHandle]int)
				for _, term := range queryTerms {
					counts[term]++
				}
				for _, term := range terms {
					counts[term]--
				}
				for _, count := range counts {
					if count != 0 {
						t.Fatal("asserted query premise does not contain exactly the original query atoms")
					}
				}
				queryPremises++
			}
		}
		parents, err := m.GetProofNumParents(node)
		queryMust(t, err)
		for i := uint32(0); i < parents; i++ {
			parent, err := m.GetProofParent(node, i)
			queryMust(t, err)
			todo = append(todo, parent)
		}
	}
	if resolutions != len(facts) || queryPremises != 1 {
		t.Fatal("proof did not use exactly the source resolution steps", resolutions, queryPremises)
	}
	for _, found := range wantFacts {
		if !found {
			t.Fatal("proof omitted an original fact")
		}
	}
}

func TestTabExpressionSourceCompoundProofAndQueryIsolation(t *testing.T) {
	f := newTabExpressionSourceFixture(t)
	_, p := f.predicate("tab-expression-source-P")
	_, q := f.predicate("tab-expression-source-Q")
	_, missing := f.predicate("tab-expression-source-Missing")
	queryMust(t, f.engine.AddRule(p, "P-fact"))
	queryMust(t, f.engine.AddRule(q, "Q-fact"))
	if md, err := f.engine.ExpressionModel(); md != nil || err == nil {
		t.Fatal("pre-query expression model snapshot", md, err)
	}
	query := f.pin(f.manager.MkAnd(p, q))
	for i := 0; i < 2; i++ {
		answer := tabExpressionSourceQuery(t, f, query, lbool.True, 2)
		checkTabExpressionSourceProof(t, f, answer, []ast.ASTHandle{p, q}, query, ast.ASTHandle{})
		queryMust(t, f.manager.DelRef(&answer))
	}
	missingQuery := f.pin(f.manager.MkAnd(p, missing))
	checkMissing := func() {
		// Query-rule normalization visits Missing before P, so the absent
		// predicate exhausts the search without unfolding a program rule.
		answer := tabExpressionSourceQuery(t, f, missingQuery, lbool.False, 0)
		h, err := f.manager.RefGet(answer)
		queryMust(t, err)
		if !f.manager.IsTrue(h) {
			t.Fatal("unreachable TAB answer is not true")
		}
		queryMust(t, f.manager.DelRef(&answer))
	}
	checkMissing()
	predicates, err := f.engine.Predicates()
	queryMust(t, err)
	rules, err := f.engine.Rules()
	queryMust(t, err)
	queryMust(t, closeQueryTestRefs(f.manager, rules))
	if len(predicates) != 3 || len(rules) != 2 {
		t.Fatal("expression queries changed the persistent program", len(predicates), len(rules))
	}
	queryMust(t, f.engine.AddRule(missing, "new-fact"))
	if md, err := f.engine.ExpressionModel(); md != nil || err == nil {
		t.Fatal("stale expression model snapshot", md, err)
	}
	if answer, err := f.engine.ExpressionAnswer(); answer != (ast.ASTRef{}) || err == nil {
		t.Fatal("stale expression answer snapshot", answer, err)
	}
	answer := tabExpressionSourceQuery(t, f, missingQuery, lbool.True, 2)
	checkTabExpressionSourceProof(t, f, answer, []ast.ASTHandle{p, missing}, missingQuery, ast.ASTHandle{})
	queryMust(t, f.manager.DelRef(&answer))
}

func TestTabExpressionSourceExistentialProof(t *testing.T) {
	f := newTabExpressionSourceFixture(t)
	m := f.manager
	boolean := f.pin(m.MkBoolSort())
	relation, _ := f.predicate("tab-expression-source-R", boolean)
	_, ready := f.predicate("tab-expression-source-Ready")
	truth := f.pin(m.MkTrue())
	fact := f.pin(m.MkApp(relation, []ast.ASTHandle{truth}))
	queryMust(t, f.engine.AddRule(fact, "R-fact"))
	queryMust(t, f.engine.AddRule(ready, "Ready-fact"))
	x := f.pin(m.MkVar(0, boolean))
	body := f.pin(m.MkAnd(f.pin(m.MkApp(relation, []ast.ASTHandle{x})), ready))
	query := f.pin(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	answer := tabExpressionSourceQuery(t, f, query, lbool.True, 2)
	checkTabExpressionSourceProof(t, f, answer, []ast.ASTHandle{fact, ready}, body, boolean)
	queryMust(t, m.DelRef(&answer))
}
