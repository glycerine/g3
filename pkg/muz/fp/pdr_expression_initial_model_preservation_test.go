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

// Source model_node::get_rule selects an initial rule from the existing model
// before considering another solver check (pdr_context.cpp:777–794).
func TestPDRExpressionAnswersPreserveInitialModelAfterPrequeryTransfer(t *testing.T) {
	for _, trace := range []bool{false, true} {
		t.Run(fmt.Sprintf("proof=%t", trace), func(t *testing.T) {
			var budget *arena.Budget
			t.Cleanup(func() {
				if budget != nil && budget.Used() != 0 {
					t.Fatal("initial-model answer leaked storage", budget.Used())
				}
			})
			m, g := newPDRLifecycleServices(t)
			budget = m.Budget()
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{"inline_eager", "inline_linear", "slice", "subsumption_checker", "coalesce_rules"} {
				must(g.SourceSetName("fixedpoint.xform."+name, "false"))
			}
			must(g.SourceSetName("fixedpoint.xform.unfold_rules", "0"))
			must(g.SourceSetName("fixedpoint.pdr.cache_mode", "0"))
			must(g.SourceSetName("fixedpoint.pdr.validate_result", "false"))
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
			name, err := m.Symbols.Intern("transferred-initial-model-P")
			must(err)
			predicate := own(m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
			must(e.RegisterPredicate(predicate))
			query := own(m.MkApp(predicate, []ast.ASTHandle{own(m.MkFalse())}))
			alternative := own(m.MkApp(predicate, []ast.ASTHandle{own(m.MkTrue())}))
			must(e.addRuleEager(query, "false-fact"))
			must(e.addRuleEager(alternative, "true-fact"))
			_, err = e.NumLevels(predicate)
			must(err)
			prequery := e.pdr
			if prequery == nil {
				t.Fatal("level lookup did not create the prequery context")
			}
			status, err := e.QueryExpression(query)
			must(err)
			if status != lbool.True || e.pdr != nil || e.queryClone == nil || e.queryClone.pdr != prequery {
				t.Fatal("expression query did not transfer the prequery context", status)
			}
			rules, err := e.queryClone.pdrRules.PredicateRules(predicate)
			must(err)
			count := len(rules)
			for _, rule := range rules {
				must(rule.Close())
			}
			if count != 2 {
				t.Fatal("preprocessing changed the two initial rules", count)
			}
			checks := func() uint32 {
				t.Helper()
				snapshot, err := e.queryClone.pdr.SnapshotStatistics()
				must(err)
				var count uint32
				for i := 0; i < snapshot.Solvers.Size(); i++ {
					if snapshot.Solvers.Key(i) == "num checks" && snapshot.Solvers.IsUint(i) {
						count += snapshot.Solvers.UintValue(i)
					}
				}
				return count
			}
			before := checks()
			answer, err := e.ExpressionAnswer()
			must(err)
			pins = append(pins, answer)
			if after := checks(); after != before {
				t.Fatal("answer rechecked an initial rule with an existing model", before, after)
			}
			references := m.Stats().LiveReferences
			repeated, err := e.ExpressionAnswer()
			must(err)
			must(m.DelRef(&repeated))
			if after := checks(); after != before || m.Stats().LiveReferences != references {
				t.Fatal("repeated answer changed solver checks or reference ownership", before, after, references, m.Stats().LiveReferences)
			}

			handle, err := m.RefGet(answer)
			must(err)
			queryHead := own(m.MkApp(e.queryClonePredicate, nil))
			bridge := own(m.MkImplies(query, queryHead))
			premises := []ast.ASTHandle{query, alternative, bridge}
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
			check(premises, lbool.True)
			isProof, err := m.IsProof(handle)
			must(err)
			if isProof != trace {
				t.Fatal("answer kind disagrees with proof-trace request", isProof, trace)
			}
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
				t.Fatal("initial-model answer proof failed structural validation")
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
		})
	}
}
