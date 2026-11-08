package fp

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Distinct nullary predicates share the true search cube. A cached leaf must
// use a donor from its own predicate, or select a feasible initial rule there.
func TestPDRExpressionCachedAnswersPreservePredicateAndInitialRule(t *testing.T) {
	for cache := 0; cache <= 2; cache++ {
		for _, trace := range []bool{false, true} {
			for _, alternatives := range []bool{false, true} {
				t.Run(fmt.Sprintf("cache=%d/proof=%t/alternatives=%t", cache, trace, alternatives), func(t *testing.T) {
					testPDRExpressionCachedAnswer(t, cache, trace, alternatives)
				})
			}
		}
	}
}

func testPDRExpressionCachedAnswer(t *testing.T, cache int, trace, alternatives bool) {
	t.Helper()
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("cached expression answer leaked storage", budget.Used())
		}
	})
	m, g := newPDRLifecycleServices(t)
	budget = m.Budget()
	phase := "fixture"
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(phase, err)
		}
	}
	for _, name := range []string{"inline_eager", "inline_linear", "slice", "subsumption_checker", "coalesce_rules"} {
		must(g.SourceSetName("fixedpoint.xform."+name, "false"))
	}
	must(g.SourceSetName("fixedpoint.xform.unfold_rules", "0"))
	must(g.SourceSetName("fixedpoint.pdr.cache_mode", fmt.Sprint(cache)))
	must(g.SourceSetName("fixedpoint.generate_proof_trace", fmt.Sprint(trace)))
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	must(err)
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("pdr"))
	var pins []ast.ASTRef
	defer func() {
		for i := len(pins) - 1; i >= 0; i-- {
			must(m.DelRef(&pins[i]))
		}
	}()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(err)
		r, err := m.NewRef(h)
		must(err)
		pins = append(pins, r)
		return h
	}
	boolean := own(m.MkBoolSort())
	var domain []ast.ASTHandle
	var values []ast.ASTHandle
	if alternatives {
		for i := 0; i < 3; i++ {
			values = append(values, own(m.ArithmeticIntegerNumeralText(fmt.Sprint(i))))
		}
		integer, err := m.GetSort(values[0])
		must(err)
		domain = []ast.ASTHandle{integer}
	}
	name, err := m.Symbols.Intern("cached-expression-P")
	must(err)
	predicate := own(m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{}))
	must(e.RegisterPredicate(predicate))
	var originals []ast.ASTHandle
	if alternatives {
		// P(0) and P(2) are infeasible for the requested state P(1). All
		// three are real initial rules, so the cache-hit fallback must use
		// the level-zero solver to choose the compatible one.
		for i, value := range values {
			fact := own(m.MkApp(predicate, []ast.ASTHandle{value}))
			originals = append(originals, fact)
			must(e.addRuleEager(fact, fmt.Sprintf("initial-%d", i)))
		}
	} else {
		fact := own(m.MkApp(predicate, nil))
		originals = append(originals, fact)
		must(e.addRuleEager(fact, "initial-nullary"))
	}
	query, replacement := originals[0], originals[0]
	if alternatives {
		query = originals[1]
	}
	_, err = e.NumLevels(predicate)
	must(err)
	prequery := e.pdr
	if prequery == nil {
		t.Fatal("level lookup did not create the prequery context")
	}
	phase = "first query"
	status, err := e.QueryExpression(query)
	must(err)
	if status != lbool.True || e.pdr != nil || e.queryClone == nil || e.queryClone.pdr != prequery {
		t.Fatal("expression query did not transfer the prequery owner", status)
	}
	if alternatives {
		rules, err := e.queryClone.pdrRules.PredicateRules(predicate)
		must(err)
		count := len(rules)
		for _, rule := range rules {
			must(rule.Close())
		}
		if count != 3 {
			t.Fatal("preprocessing removed the initial-rule selection exercise", count)
		}
	}
	phase = "first answer"
	answer, err := e.ExpressionAnswer()
	must(err)
	pins = append(pins, answer)
	handle, err := m.RefGet(answer)
	must(err)
	queryHead := own(m.MkApp(e.queryClonePredicate, nil))
	bridge := own(m.MkImplies(query, queryHead))
	independent, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: g.Context, Globals: g})
	must(err)
	defer func() { must(independent.Close()) }()
	check := func(formulas []ast.ASTHandle, want lbool.LBool) {
		t.Helper()
		must(independent.Push())
		for _, formula := range formulas {
			must(independent.Assert(formula))
		}
		actual, err := independent.CheckSat(nil)
		must(err)
		if actual != want {
			t.Fatal("independent answer validation", actual, "want", want)
		}
		must(independent.Pop(1))
	}
	validate := func() {
		t.Helper()
		isProof, err := m.IsProof(handle)
		must(err)
		if isProof != trace {
			t.Fatal("answer kind disagrees with proof-trace request", isProof, trace)
		}
		premises := append(append([]ast.ASTHandle(nil), originals...), bridge)
		check(premises, lbool.True)
		if !trace {
			check(append(premises, handle), lbool.True)
			check([]ast.ASTHandle{handle, own(m.MkNot(query))}, lbool.False)
			check([]ast.ASTHandle{handle, own(m.MkNot(queryHead))}, lbool.False)
			return
		}
		conclusion, err := m.GetProofFact(handle)
		must(err)
		if conclusion != queryHead {
			t.Fatal("proof concludes a different query predicate")
		}
		checker, err := proofs.NewChecker(m)
		must(err)
		valid, conditions, err := checker.Check(handle)
		must(err)
		defer func() {
			for _, condition := range conditions {
				must(m.DecRef(condition))
			}
		}()
		if !valid {
			t.Fatal("cached answer proof failed structural validation")
		}
		for _, condition := range conditions {
			check([]ast.ASTHandle{own(m.MkNot(condition))}, lbool.False)
		}
		seen := make(map[ast.ASTHandle]bool)
		stack, leaves := []ast.ASTHandle{handle}, 0
		for len(stack) != 0 {
			node := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if seen[node] {
				continue
			}
			seen[node] = true
			asserted, err := m.IsProofKind(node, ast.ASTPrAsserted)
			must(err)
			if asserted {
				leaves++
				premise, err := m.GetProofFact(node)
				must(err)
				check(append(premises, own(m.MkNot(premise))), lbool.False)
			}
			parents, err := m.GetProofNumParents(node)
			must(err)
			for i := uint32(0); i < parents; i++ {
				parent, err := m.GetProofParent(node, i)
				must(err)
				stack = append(stack, parent)
			}
		}
		if leaves == 0 {
			t.Fatal("proof has no independently justified premises")
		}
	}
	phase = "first independent validation"
	validate()
	before := m.Stats().LiveReferences
	phase = "repeated answer"
	repeated, err := e.ExpressionAnswer()
	must(err)
	must(m.DelRef(&repeated))
	if m.Stats().LiveReferences != before {
		t.Fatal("repeated answer construction changed reference ownership")
	}
	phase = "replacement query"
	status, err = e.QueryExpression(replacement)
	must(err)
	if status != lbool.True || e.queryClone == nil || e.queryClone.pdr == prequery {
		t.Fatal("replacement query did not replace the old context", status)
	}
	if _, err = prequery.Initialized(); err == nil {
		t.Fatal("replacement left the prior context open")
	}
	held, err := m.RefGet(answer)
	must(err)
	if held != handle {
		t.Fatal("query replacement changed the caller's held answer")
	}
	phase = "held answer validation"
	validate()
}
