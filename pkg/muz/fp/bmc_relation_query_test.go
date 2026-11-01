package fp

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBMCRelationQueryUsesSourceVariableOrder(t *testing.T) {
	for _, test := range []struct {
		backend string
		forced  bool
	}{{"bmc", false}, {"bmc", true}, {"qbmc", false}, {"qbmc", true}} {
		name := test.backend + "-parameter"
		if test.forced {
			name = test.backend + "-forced"
		}
		t.Run(name, func(t *testing.T) {
			manager, globals := newPDRLifecycleServices(t)
			if !test.forced {
				fragmentMust(t, globals.SourceSetName("fixedpoint.generate_proof_trace", "true"))
			}
			engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, engine.Close()) }()
			engine.forceProofTrace = test.forced
			fragmentMust(t, engine.SelectEngine(test.backend))
			boolean, err := manager.MkBoolSort()
			fragmentMust(t, err)
			zero, err := manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
			fragmentMust(t, err)
			integer, err := manager.GetSort(zero)
			fragmentMust(t, err)
			name, err := manager.Symbols.Intern("bmc-mixed-query-" + test.backend)
			fragmentMust(t, err)
			predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
			fragmentMust(t, err)
			fragmentMust(t, engine.RegisterPredicate(predicate))
			truth, err := manager.MkTrue()
			fragmentMust(t, err)
			fact, err := manager.MkApp(predicate, []ast.ASTHandle{truth, zero})
			fragmentMust(t, err)
			fragmentMust(t, engine.AddRule(fact, "mixed-fact"))
			status, err := engine.QueryPredicate(predicate)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("mixed-sort relation query", status)
			}
			proof, err := manager.RefGet(engine.bmcProof)
			fragmentMust(t, err)
			conclusion, err := manager.GetProofFact(proof)
			fragmentMust(t, err)
			wantConclusion, err := manager.MkApp(engine.queryClonePredicate, []ast.ASTHandle{zero, truth})
			fragmentMust(t, err)
			if conclusion != wantConclusion {
				t.Fatal("mixed-sort certificate has incorrect conclusion")
			}
			checker, err := proofs.NewChecker(manager)
			fragmentMust(t, err)
			valid, conditions, err := checker.Check(proof)
			fragmentMust(t, err)
			defer func() {
				for _, condition := range conditions {
					fragmentMust(t, manager.DecRef(condition))
				}
			}()
			if !valid {
				t.Fatal("invalid mixed-sort query certificate")
			}
			solver, err := smtcore.NewSMTSolver(manager, nil, "", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
			fragmentMust(t, err)
			defer func() { fragmentMust(t, solver.Close()) }()
			for _, condition := range conditions {
				fragmentMust(t, solver.Push())
				negated, err := manager.MkNot(condition)
				fragmentMust(t, err)
				fragmentMust(t, solver.AssertFormula(negated))
				status, err := solver.CheckSat(nil)
				fragmentMust(t, err)
				fragmentMust(t, solver.Pop(1))
				if status != lbool.False {
					t.Fatal("invalid certificate side condition", status)
				}
			}
			for i, want := range []ast.ASTHandle{integer, boolean} {
				got, err := manager.GetDomain(engine.queryClonePredicate, uint32(i))
				fragmentMust(t, err)
				if got != want {
					t.Fatalf("query head domain %d has incorrect sort", i)
				}
			}
			rules, err := engine.queryClone.set.PredicateRules(engine.queryClonePredicate)
			fragmentMust(t, err)
			if len(rules) != 1 {
				t.Fatal("synthetic query rule count", len(rules))
			}
			defer func() { fragmentMust(t, rules[0].Close()) }()
			head, err := rules[0].Head()
			fragmentMust(t, err)
			tail, negative, err := rules[0].Tail(0)
			fragmentMust(t, err)
			if negative {
				t.Fatal("synthetic query has a negative relation tail")
			}
			for i := uint32(0); i < 2; i++ {
				headArg, err := manager.GetArg(head, i)
				fragmentMust(t, err)
				headIndex, err := manager.GetVarIndex(headArg)
				fragmentMust(t, err)
				tailArg, err := manager.GetArg(tail, i)
				fragmentMust(t, err)
				tailIndex, err := manager.GetVarIndex(tailArg)
				fragmentMust(t, err)
				if headIndex != 1-i || tailIndex != i {
					t.Fatalf("argument %d: head variable %d, tail variable %d", i, headIndex, tailIndex)
				}
			}
		})
	}
}
