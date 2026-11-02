package fp

// Source: src/muz/spacer/spacer_context.cpp get_ground_sat_answer.
// Local counterexample models must preserve parent constraints across branches.

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerSymbolicNonlinearWitnessOriginalClauses(t *testing.T) {
	for _, sliced := range []bool{false, true} {
		name := "plain"
		if sliced {
			name = "slice"
		}
		t.Run(name, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			for _, key := range []string{"inline_eager", "inline_linear", "slice", "subsumption_checker", "coalesce_rules"} {
				fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "false"))
			}
			fragmentMust(t, g.SourceSetName("fixedpoint.xform.unfold_rules", "0"))
			if sliced {
				fragmentMust(t, g.SourceSetName("fixedpoint.xform.slice", "true"))
			}
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
			decl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				s, err := m.Symbols.Intern(name)
				fragmentMust(t, err)
				d := own(m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{}))
				fragmentMust(t, e.RegisterPredicate(d))
				return d
			}
			goal := decl("numeric-goal", nil)
			left := decl("numeric-left", []ast.ASTHandle{integer})
			right := decl("numeric-right", []ast.ASTHandle{integer})
			leaf := decl("numeric-leaf", []ast.ASTHandle{integer, integer})
			app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle { return own(m.MkApp(d, args)) }
			and := func(args ...ast.ASTHandle) ast.ASTHandle { return own(m.MkAndArray(args)) }
			var originals []ast.ASTHandle
			add := func(body, head ast.ASTHandle, n int, name string) {
				formula := own(m.MkImplies(body, head))
				sorts := make([]ast.ASTHandle, n)
				names := make([]symbol.Symbol, n)
				for i := range sorts {
					sorts[i] = integer
					names[i] = symbol.Numerical(uint32(i))
				}
				if n != 0 {
					formula = own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: formula}))
				}
				originals = append(originals, formula)
				fragmentMust(t, e.AddRule(formula, name))
			}
			// The unused leaf column forces real slicing, while the numeric column
			// must preserve the distinct left/right instances and their correlation.
			add(and(own(m.MkArithmeticGE(x, two)), own(m.MkArithmeticLE(x, five))), app(leaf, x, y), 2, "numeric-leaf-range")
			add(and(app(leaf, x, y), own(m.MkArithmeticLE(x, three))), app(left, x), 2, "numeric-left-step")
			add(and(app(leaf, x, y), own(m.MkArithmeticGE(x, four))), app(right, x), 2, "numeric-right-step")
			add(and(app(left, x), app(right, y), own(m.MkEq(y, own(m.MkArithmeticAdd(x, one))))), app(goal), 2, "numeric-goal-step")
			solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: g.Context, Globals: g})
			fragmentMust(t, err)
			defer func() { fragmentMust(t, solver.Close()) }()
			for repeat := 0; repeat < 2; repeat++ {
				status, err := e.QueryPredicate(goal)
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("symbolic nonlinear query", status)
				}
				forms, names, err := e.RulesAlongTrace()
				fragmentMust(t, err)
				occurrences := map[string]int{}
				for i, name := range names {
					occurrences[name]++
					fragmentMust(t, m.DelRef(&forms[i]))
				}
				wantNames := map[string]int{"": 1, "numeric-goal-step": 1, "numeric-left-step": 1, "numeric-right-step": 1, "numeric-leaf-range": 2}
				if !reflect.DeepEqual(occurrences, wantNames) {
					t.Fatal("original named-rule occurrences", occurrences, wantNames)
				}
				answer, err := e.GroundSatAnswer(goal)
				fragmentMust(t, err)
				h, err := m.RefGet(answer)
				fragmentMust(t, err)
				count, err := m.GetNumArgs(h)
				fragmentMust(t, err)
				if count != 5 {
					t.Fatal("expected goal, two branches, and two leaf instances", count)
				}
				var leftValue, rightValue ast.ASTHandle
				leafValues := map[ast.ASTHandle]int{}
				mappedLeaf := e.spacerPredicate(leaf)
				if sliced && !slicedPredicateHasRemovedColumns(e.spacerSlices, leaf) {
					t.Fatal("fixture did not slice unused column")
				}
				fragmentMust(t, solver.Push())
				for _, formula := range originals {
					fragmentMust(t, solver.Assert(formula))
				}
				status, err = solver.CheckSat(nil)
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("original clauses are inconsistent", status)
				}
				for i := uint32(0); i < count; i++ {
					fact, err := m.GetArg(h, i)
					fragmentMust(t, err)
					d, err := m.GetDecl(fact)
					fragmentMust(t, err)
					originalFact := fact
					switch d {
					case left:
						leftValue, err = m.GetArg(fact, 0)
						fragmentMust(t, err)
					case right:
						rightValue, err = m.GetArg(fact, 0)
						fragmentMust(t, err)
					case mappedLeaf:
						value, err := m.GetArg(fact, 0)
						fragmentMust(t, err)
						leafValues[value]++
						if sliced {
							originalFact = app(leaf, value, one)
						}
					case goal:
					default:
						t.Fatal("unexpected exported predicate", d)
					}
					fragmentMust(t, solver.Push())
					fragmentMust(t, solver.Assert(own(m.MkNot(originalFact))))
					status, err := solver.CheckSat(nil)
					fragmentMust(t, err)
					if status != lbool.False {
						t.Fatal("original clauses do not imply exported witness", i, status)
					}
					fragmentMust(t, solver.Pop(1))
				}
				fragmentMust(t, solver.Pop(1))
				if leftValue.IsNull() || rightValue.IsNull() || len(leafValues) != 2 || leafValues[leftValue] != 1 || leafValues[rightValue] != 1 {
					t.Fatal("branch and leaf instances disagree", leftValue, rightValue, leafValues)
				}
				// Check numeric correlation independently, without Horn clauses that could
				// hide an inconsistent reconstructed branch behind an inconsistent model.
				condition := and(own(m.MkArithmeticGE(leftValue, two)), own(m.MkArithmeticLE(leftValue, three)), own(m.MkArithmeticGE(rightValue, four)), own(m.MkArithmeticLE(rightValue, five)), own(m.MkEq(rightValue, own(m.MkArithmeticAdd(leftValue, one)))))
				fragmentMust(t, solver.Push())
				fragmentMust(t, solver.Assert(own(m.MkNot(condition))))
				status, err = solver.CheckSat(nil)
				fragmentMust(t, err)
				if status != lbool.False {
					t.Fatal("ground values violate original branch constraints", status)
				}
				fragmentMust(t, solver.Pop(1))
				fragmentMust(t, m.DelRef(&answer))
			}
		})
	}
}
