package fp

// Sources: spacer_context.cpp get_ground_sat_answer queue traversal,
// dl_mk_slice.cpp actual column projection, dl_mk_unfold.cpp expand_tail,
// and dl_mk_coalesce.cpp representative rule names.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerSlicedUnfoldedSharedWitnessOriginalClauses(t *testing.T) {
	for _, rounds := range []int{1, 2} {
		for _, coalesce := range []bool{false, true} {
			t.Run(fmt.Sprintf("rounds=%d/coalesce=%t", rounds, coalesce), func(t *testing.T) {
				m, g := newPDRLifecycleServices(t)
				for _, key := range []string{"inline_eager", "inline_linear", "subsumption_checker"} {
					fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "false"))
				}
				fragmentMust(t, g.SourceSetName("fixedpoint.xform.slice", "true"))
				fragmentMust(t, g.SourceSetName("fixedpoint.xform.unfold_rules", fmt.Sprint(rounds)))
				fragmentMust(t, g.SourceSetName("fixedpoint.xform.coalesce_rules", fmt.Sprint(coalesce)))
				e, err := NewPropositionalEngineWithServices(m, g.Context, g)
				fragmentMust(t, err)
				defer func() { fragmentMust(t, e.Close()) }()
				fragmentMust(t, e.SelectEngine("spacer"))
				var pins []ast.ASTRef
				defer func() {
					for i := len(pins) - 1; i >= 0; i-- {
						fragmentMust(t, m.DelRef(&pins[i]))
					}
				}()
				own := func(h ast.ASTHandle, err error) ast.ASTHandle {
					fragmentMust(t, err)
					r, err := m.NewRef(h)
					fragmentMust(t, err)
					pins = append(pins, r)
					return h
				}
				boolean := own(m.MkBoolSort())
				one := own(m.ArithmeticIntegerNumeralText("1"))
				two := own(m.ArithmeticIntegerNumeralText("2"))
				three := own(m.ArithmeticIntegerNumeralText("3"))
				four := own(m.ArithmeticIntegerNumeralText("4"))
				five := own(m.ArithmeticIntegerNumeralText("5"))
				integer, err := m.GetSort(one)
				fragmentMust(t, err)
				x := own(m.MkVar(0, integer))
				y := own(m.MkVar(1, integer))
				z := own(m.MkVar(2, integer))
				w := own(m.MkVar(3, integer))
				arbitrary := own(m.MkFreshConst("sliced-away-value", integer))
				declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
					s, err := m.Symbols.Intern(name)
					fragmentMust(t, err)
					d := own(m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{}))
					fragmentMust(t, e.RegisterPredicate(d))
					return d
				}
				app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle { return own(m.MkApp(d, args)) }
				and := func(args ...ast.ASTHandle) ast.ASTHandle { return own(m.MkAndArgs(args)) }
				shared := make([]ast.ASTHandle, 9)
				goals := make([]ast.ASTHandle, 9)
				for i := range shared {
					shared[i] = declare(fmt.Sprintf("slice-unfold-shared-%d", i), []ast.ASTHandle{integer, integer})
					goals[i] = declare(fmt.Sprintf("slice-unfold-goal-%d", i), nil)
				}
				var originals []ast.ASTHandle
				add := func(body, head ast.ASTHandle, vars int, name string) {
					formula := own(m.MkImplies(body, head))
					if vars > 0 {
						sorts := make([]ast.ASTHandle, vars)
						names := make([]symbol.Symbol, vars)
						for i := range sorts {
							sorts[i] = integer
							names[i] = symbol.Numerical(uint32(i))
						}
						formula = own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: formula}))
					}
					originals = append(originals, formula)
					fragmentMust(t, e.addRuleEager(formula, name))
				}
				// Every shared relation carries a second integer column that never
				// affects a guard or the query. Distinct z/w branch arguments keep it
				// unconstrained while x/y enforce the nonlinear numeric correlation.
				add(and(own(m.MkArithmeticGE(x, two)), own(m.MkArithmeticLE(x, three))), app(shared[8], x, y), 2, "leaf-low")
				add(and(own(m.MkArithmeticGE(x, four)), own(m.MkArithmeticLE(x, five))), app(shared[8], x, y), 2, "leaf-high")
				for i := 0; i < 8; i++ {
					add(app(shared[i+1], x, y), app(shared[i], x, y), 2, fmt.Sprintf("shared-%d", i))
				}
				add(and(app(shared[0], x, z), app(shared[0], y, w), own(m.MkArithmeticLE(x, three)), own(m.MkArithmeticGE(y, four)), own(m.MkEq(y, own(m.MkArithmeticAdd(x, one))))), app(goals[0]), 4, "nonlinear-goal")
				for i := 1; i < 9; i++ {
					add(app(goals[i-1]), app(goals[i]), 0, fmt.Sprintf("goal-%d", i))
				}
				solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: g.Context, Globals: g})
				fragmentMust(t, err)
				defer func() { fragmentMust(t, solver.Close()) }()
				inputCount, err := e.set.Len()
				fragmentMust(t, err)
				var exports []ast.ASTRef
				var exportedHandles []ast.ASTHandle
				defer func() {
					for i := len(exports) - 1; i >= 0; i-- {
						fragmentMust(t, m.DelRef(&exports[i]))
					}
				}()
				holdExport := func(reference ast.ASTRef) ast.ASTHandle {
					handle, err := m.RefGet(reference)
					fragmentMust(t, err)
					exports = append(exports, reference)
					exportedHandles = append(exportedHandles, handle)
					return handle
				}
				for repeat := 0; repeat < 2; repeat++ {
					status, err := e.QueryPredicate(goals[8])
					fragmentMust(t, err)
					if status != lbool.True {
						t.Fatal("symbolic query", status)
					}
					afterCount, err := e.set.Len()
					fragmentMust(t, err)
					if afterCount != inputCount {
						t.Fatal("query changed the original rule set", afterCount, inputCount)
					}
					originalPredicate := make(map[ast.ASTHandle]ast.ASTHandle)
					for _, declaration := range shared {
						removed := e.spacerSlices[declaration]
						if len(removed) != 2 || removed[0] || !removed[1] {
							t.Fatal("fixture did not project exactly the dead integer column", removed)
						}
						derived := e.spacerPredicate(declaration)
						arity, err := m.GetArity(derived)
						fragmentMust(t, err)
						if derived == declaration || arity != 1 {
							t.Fatal("projected predicate kept its original declaration or arity", arity)
						}
						originalPredicate[derived] = declaration
					}
					forms, names, err := e.RulesAlongTrace()
					fragmentMust(t, err)
					leafCount := 0
					lowCount, highCount := 0, 0
					unfolded := false
					for i, name := range names {
						unfolded = unfolded || strings.Contains(name, ";")
						for _, part := range strings.Split(name, ";") {
							if part == "leaf-low" || part == "leaf-high" {
								leafCount++
								if part == "leaf-low" {
									lowCount++
								} else {
									highCount++
								}
							}
						}
						holdExport(forms[i])
					}
					if leafCount != 2 {
						t.Fatal("trace lost duplicate shared leaf derivations", leafCount, names)
					}
					if !unfolded {
						t.Fatal("fixture did not unfold any trace rule")
					}
					// Source coalescing keeps its first representative's name; the
					// merged base must still occur independently on both branches.
					if coalesce {
						if lowCount != 2 || highCount != 0 {
							t.Fatal("fixture did not coalesce the two base cases", names)
						}
					} else if lowCount != 1 || highCount != 1 {
						t.Fatal("unfolded trace lost an original base case", names)
					}
					answer, err := e.GroundSatAnswer(goals[8])
					fragmentMust(t, err)
					h := holdExport(answer)
					count, err := m.GetNumArgs(h)
					fragmentMust(t, err)
					if count < 4 {
						t.Fatal("unfolding collapsed the nonlinear fixture", count)
					}
					fragmentMust(t, solver.Push())
					for _, rule := range originals {
						fragmentMust(t, solver.Assert(rule))
					}
					status, err = solver.CheckSat(nil)
					fragmentMust(t, err)
					if status != lbool.True {
						t.Fatal("inconsistent original clauses", status)
					}
					var leaves []ast.ASTHandle
					for i := uint32(0); i < count; i++ {
						fact, err := m.GetArg(h, i)
						fragmentMust(t, err)
						d, err := m.GetDecl(fact)
						fragmentMust(t, err)
						if d == e.spacerPredicate(shared[8]) {
							value, err := m.GetArg(fact, 0)
							fragmentMust(t, err)
							leaves = append(leaves, value)
						}
						originalFact := fact
						if original, projected := originalPredicate[d]; projected {
							arity, err := m.GetNumArgs(fact)
							fragmentMust(t, err)
							if arity != 1 {
								t.Fatal("exported projected fact has wrong arity", arity)
							}
							value, err := m.GetArg(fact, 0)
							fragmentMust(t, err)
							// The source exports sliced predicates. Restore the dead column
							// with an arbitrary integer to check against original clauses.
							originalFact = app(original, value, arbitrary)
						}
						fragmentMust(t, solver.Push())
						fragmentMust(t, solver.Assert(own(m.MkNot(originalFact))))
						status, err = solver.CheckSat(nil)
						fragmentMust(t, err)
						if status != lbool.False {
							t.Fatal("original clauses do not imply exported fact", i, status)
						}
						fragmentMust(t, solver.Pop(1))
					}
					fragmentMust(t, solver.Pop(1))
					if len(leaves) != 2 {
						t.Fatal("missing repeated shared predicate instances", len(leaves))
					}
					correlation := and(own(m.MkEq(leaves[0], three)), own(m.MkEq(leaves[1], four)), own(m.MkEq(leaves[1], own(m.MkArithmeticAdd(leaves[0], one)))))
					fragmentMust(t, solver.Push())
					fragmentMust(t, solver.Assert(own(m.MkNot(correlation))))
					status, err = solver.CheckSat(nil)
					fragmentMust(t, err)
					if status != lbool.False {
						t.Fatal("shared instances violate original branch correlation", status)
					}
					fragmentMust(t, solver.Pop(1))
					// A new source-style virtual solver can add factory-owned guards,
					// so validate exported owners across reuse and final engine disposal.
					repeated, err := e.GroundSatAnswer(goals[8])
					fragmentMust(t, err)
					if repeatedHandle, err := m.RefGet(repeated); err != nil || repeatedHandle != h {
						t.Fatal("repeated ground witness changed", err)
					}
					fragmentMust(t, m.DelRef(&repeated))
				}
				fragmentMust(t, e.Close())
				for i, reference := range exports {
					if handle, err := m.RefGet(reference); err != nil || handle != exportedHandles[i] {
						t.Fatal("prior witness or rule export lost ownership after requery and engine close", i, err)
					}
				}
			})
		}
	}
}
