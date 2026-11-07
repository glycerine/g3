package pdr_test

// Source: model_search::get_proof_trace caches completed rules and proofs by
// state in src/muz/pdr/pdr_context.cpp. Duplicate search leaves can denote an
// already expanded noninitial obligation rather than an initial fact.

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestContextProofSharesNoninitialDiamondState(t *testing.T) {
	for _, nullary := range []bool{false, true} {
		for _, breadth := range []bool{true, false} {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("nullary=%t/breadth=%t/reverse=%t", nullary, breadth, reverse), func(t *testing.T) {
					command, err := cmd_context.NewStandaloneContext()
					pdrMust(t, err)
					defer func() { pdrMust(t, command.Close()) }()
					manager, err := command.Manager()
					pdrMust(t, err)
					globals, err := command.GlobalParameters()
					pdrMust(t, err)
					config := smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals}
					context, err := pdr.NewContext(manager, nil, 4, pdr.ReachableHashCache, config)
					pdrMust(t, err)
					defer func() { pdrMust(t, context.Close()) }()
					pdrMust(t, context.SetGenerateProofTrace(true))
					pdrMust(t, context.SetBreadthFirstModelSearch(breadth))
					boolean, err := manager.MkBoolSort()
					pdrMust(t, err)
					domain := []ast.ASTHandle{boolean}
					if nullary {
						domain = nil
					}
					declarations := make([]ast.ASTHandle, 4)
					for index, name := range []string{"shared_a", "shared_p", "shared_q", "shared_r"} {
						declarations[index] = pdrFunction(t, manager, name, domain, boolean)
					}
					goal := pdrFunction(t, manager, "shared_goal", nil, boolean)
					rules, err := base.NewRuleManager(manager)
					pdrMust(t, err)
					defer func() { pdrMust(t, rules.Close()) }()
					for _, declaration := range append(declarations, goal) {
						pdrMust(t, rules.RegisterPredicate(declaration))
					}
					set, err := base.NewRuleSet(rules)
					pdrMust(t, err)
					defer func() { pdrMust(t, set.Close()) }()
					var pins []ast.ASTRef
					defer func() {
						for index := len(pins) - 1; index >= 0; index-- {
							pdrMust(t, manager.DelRef(&pins[index]))
						}
					}()
					own := func(handle ast.ASTHandle, cause error) ast.ASTHandle {
						pdrMust(t, cause)
						reference, cause := manager.NewRef(handle)
						pdrMust(t, cause)
						pins = append(pins, reference)
						return handle
					}
					variable := own(manager.MkVar(0, boolean))
					truth := own(manager.MkTrue())
					app := func(declaration, argument ast.ASTHandle) ast.ASTHandle {
						var arguments []ast.ASTHandle
						if !nullary && declaration != goal {
							arguments = []ast.ASTHandle{argument}
						}
						return own(manager.MkApp(declaration, arguments))
					}
					originals := make(map[ast.ASTHandle]bool)
					add := func(head ast.ASTHandle, tail []ast.ASTHandle, name string) {
						rule, cause := rules.NewRule(head, tail, nil, name)
						pdrMust(t, cause)
						defer func() { pdrMust(t, rule.Close()) }()
						pdrMust(t, set.Add(rule))
						formula, cause := rule.ToFormula()
						pdrMust(t, cause)
						pins = append(pins, formula)
						handle, cause := manager.RefGet(formula)
						pdrMust(t, cause)
						originals[handle] = true
					}
					var initial []ast.ASTHandle
					if !nullary {
						initial = []ast.ASTHandle{own(manager.MkEq(variable, truth))}
					}
					add(app(declarations[0], variable), initial, "initial")
					add(app(declarations[1], variable), []ast.ASTHandle{app(declarations[0], variable)}, "shared-step")
					add(app(declarations[2], variable), []ast.ASTHandle{app(declarations[1], variable)}, "left")
					add(app(declarations[3], variable), []ast.ASTHandle{app(declarations[1], variable)}, "right")
					tails := []ast.ASTHandle{app(declarations[2], truth), app(declarations[3], truth)}
					if reverse {
						tails[0], tails[1] = tails[1], tails[0]
					}
					goalApp := app(goal, ast.ASTHandle{})
					add(goalApp, tails, "goal")
					pdrMust(t, context.Initialize(set))
					status, err := context.Solve(goal)
					pdrMust(t, err)
					if status != lbool.True {
						t.Fatal("shared-state diamond is reachable", status)
					}
					proof, err := context.Proof(goal)
					pdrMust(t, err)
					defer func() { pdrMust(t, manager.DelRef(&proof)) }()
					handle, err := manager.RefGet(proof)
					pdrMust(t, err)
					checker, err := proofs.NewChecker(manager)
					pdrMust(t, err)
					valid, conditions, err := checker.Check(handle)
					pdrMust(t, err)
					defer func() {
						for _, condition := range conditions {
							pdrMust(t, manager.DecRef(condition))
						}
					}()
					if !valid {
						t.Fatal("shared-state proof failed structural validation")
					}
					solver, err := smtcore.NewSMTSolver(manager, nil, "", config)
					pdrMust(t, err)
					defer func() { pdrMust(t, solver.Close()) }()
					checkValid := func(formula ast.ASTHandle) {
						// Hyper-resolution side conditions can have free rule variables.
						// Their universal closure must be valid, including Boolean ones.
						used, cause := ast.NewASTUsedVars(manager)
						pdrMust(t, cause)
						pdrMust(t, used.Process(formula))
						if count := used.MaxFoundVarIndexPlusOne(); count != 0 {
							sorts := make([]ast.ASTHandle, count)
							names := make([]symbol.Symbol, count)
							for index := uint32(0); index < count; index++ {
								sorts[count-1-index] = used.Contains(index)
								if sorts[count-1-index].IsNull() {
									sorts[count-1-index] = boolean
								}
								names[count-1-index] = symbol.Numerical(index)
							}
							formula = own(manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: formula}))
						}
						pdrMust(t, solver.Push())
						pdrMust(t, solver.Assert(own(manager.MkNot(formula))))
						status, cause := solver.CheckSat(nil)
						pdrMust(t, cause)
						if status != lbool.False {
							var text bytes.Buffer
							pdrMust(t, ast.DisplayLegacySMT(&text, manager, formula, 0, nil))
							t.Fatal("invalid proof condition or conclusion", status, text.String())
						}
						pdrMust(t, solver.Pop(1))
					}
					for _, condition := range conditions {
						checkValid(condition)
					}
					fact, err := manager.GetProofFact(handle)
					pdrMust(t, err)
					checkValid(own(manager.MkEq(fact, goalApp)))
					seen, leaves := make(map[ast.ASTHandle]bool), make(map[ast.ASTHandle]bool)
					stack := []ast.ASTHandle{handle}
					for len(stack) != 0 {
						node := stack[len(stack)-1]
						stack = stack[:len(stack)-1]
						if seen[node] {
							continue
						}
						seen[node] = true
						asserted, cause := manager.IsProofKind(node, ast.ASTPrAsserted)
						pdrMust(t, cause)
						if asserted {
							premise, cause := manager.GetProofFact(node)
							pdrMust(t, cause)
							if !originals[premise] {
								t.Fatal("proof asserted a formula outside the source rules", premise)
							}
							leaves[premise] = true
						}
						count, cause := manager.GetProofNumParents(node)
						pdrMust(t, cause)
						for index := uint32(0); index < count; index++ {
							parent, cause := manager.GetProofParent(node, index)
							pdrMust(t, cause)
							stack = append(stack, parent)
						}
					}
					if len(leaves) != len(originals) {
						t.Fatal("proof lost a source rule in the shared derivation", len(leaves), len(originals))
					}
					// Live counts must return exactly. Arena capacity may grow once for
					// reconstruction's temporary nodes, but never on a later repeat.
					reconstruct := func() {
						repeated, err := context.Proof(goal)
						pdrMust(t, err)
						pdrMust(t, manager.DelRef(&repeated))
					}
					live := func() [4]uint32 {
						stats := manager.Stats()
						return [4]uint32{stats.LiveNodes, stats.LiveParameters, stats.LiveInfos, stats.LiveReferences}
					}
					before := live()
					reconstruct()
					if live() != before {
						t.Fatal("repeated shared proof reconstruction leaked ownership", before, live())
					}
					capacity := manager.Budget().Used()
					for repeat := 0; repeat < 3; repeat++ {
						reconstruct()
						if live() != before || manager.Budget().Used() != capacity {
							t.Fatal("repeated shared proof reconstruction leaked ownership", repeat, before, live(), capacity, manager.Budget().Used())
						}
					}
				})
			}
		}
	}
}
