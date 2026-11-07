package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestSMTUTVPIContextConflictsProofsAndScopes(t *testing.T) {
	for _, parity := range []bool{false, true} {
		for _, integer := range []bool{false, true} {
			if parity && !integer {
				continue
			}
			for _, proofs := range []bool{false, true} {
				t.Run(fmt.Sprint(parity, "/", integer, "/", proofs), func(t *testing.T) {
					m := astTestManager(t)
					if proofs {
						astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
					}
					family, err := m.RegisterArithmeticPlugin()
					astTestMust(t, err)
					_, _, state := smtInternalizerFixture(t, m, nil)
					theory, err := NewSMTUTVPITheory(family)
					astTestMust(t, err)
					astTestMust(t, state.RegisterTheory(theory))
					zero, err := m.ArithmeticNumeralRat(new(big.Rat), integer)
					astTestMust(t, err)
					sort, err := m.GetSort(zero)
					astTestMust(t, err)
					x, err := m.MkConst(astTestSymbol(t, m, "utvpi-context-x"), sort)
					astTestMust(t, err)
					y, err := m.MkConst(astTestSymbol(t, m, "utvpi-context-y"), sort)
					astTestMust(t, err)
					astTestMust(t, state.PushScope())
					app := func(op int32, args ...ast.ASTHandle) ast.ASTHandle {
						result, err := m.MkFamilyApp(family, op, nil, args)
						astTestMust(t, err)
						return result
					}
					assert := func(op int32, left ast.ASTHandle, value int64) {
						n, err := m.ArithmeticNumeralRat(big.NewRat(value, 1), integer)
						astTestMust(t, err)
						formula := app(op, left, n)
						astTestMust(t, state.Internalize(formula, true))
						literal, err := state.Literal(formula)
						astTestMust(t, err)
						astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
					}
					if parity {
						sum, difference := app(smtArithOpAdd, x, y), app(smtArithOpSub, x, y)
						assert(smtArithOpLE, sum, 1)
						assert(smtArithOpGE, sum, 1)
						assert(smtArithOpLE, difference, 0)
						assert(smtArithOpGE, difference, 0)
					} else {
						assert(smtArithOpLE, x, 0)
						assert(smtArithOpGT, x, 0)
					}
					astTestMust(t, state.PropagateTheoryAssignments())
					astTestMust(t, theory.Propagate())
					var edges []int32
					if parity {
						if state.inconsistent {
							t.Fatal("integer parity contradiction was incorrectly a rational negative cycle")
						}
						edges, err = theory.graph.parityConflict()
						astTestMust(t, err)
						status, err := theory.FinalCheck()
						astTestMust(t, err)
						if status != SMTFinalCheckContinue {
							t.Fatal("parity conflict did not restart Boolean search", status)
						}
					} else {
						edges, err = theory.graph.graph.NegativeCycleEdges()
						astTestMust(t, err)
					}
					conflict, found := state.Conflict()
					if !found {
						t.Fatal("missing UTVPI conflict")
					}
					owner, _ := conflict.Justification()
					extended, ok := owner.(*SMTExtTheoryConflictJustification)
					if !ok || len(extended.Equalities) != 0 {
						t.Fatal("source extended theory conflict was not retained")
					}
					stats := statistics.New()
					astTestMust(t, theory.CollectStatistics(stats))
					counts := map[statistics.Key]uint32{}
					for i := 0; i < stats.Size(); i++ {
						if !stats.IsUint(i) {
							t.Fatal("small UTVPI count was not an unsigned statistic")
						}
						counts[stats.Key(i)] = stats.UintValue(i)
					}
					assertions := uint32(2)
					if parity {
						assertions = 4
					}
					if counts["utvpi conflicts"] != 1 || counts["utvpi asserts"] != assertions || counts["dl prop steps"] == 0 {
						t.Fatal("source conflict/assertion/graph counters", counts)
					}
					parameters := extended.Parameters()
					if !proofs && len(parameters) != 0 {
						t.Fatal("proof-disabled conflict allocated numerical parameters")
					}
					if proofs {
						if len(parameters) != len(edges)+1 {
							t.Fatal("conflict coefficient count", len(parameters), edges)
						}
						symbol, err := m.ParameterSymbol(parameters[0])
						astTestMust(t, err)
						rule, _, err := m.Symbols.BareString(symbol)
						astTestMust(t, err)
						if rule != "farkas" {
							t.Fatal("source conflict rule", rule)
						}
						for i, edge := range edges {
							coefficient := int64(2)
							if parity {
								coefficient = 1
							}
							if int64(theory.graph.coefficients[edge]) != coefficient {
								t.Fatal("source explanation factor", edge)
							}
							want, err := m.NewParameterRat(big.NewRat(coefficient, 1))
							astTestMust(t, err)
							equal, err := m.ParameterEqual(parameters[i+1], want)
							astTestMust(t, err)
							astTestMust(t, m.DelParameter(&want))
							if !equal {
								t.Fatal("owned UTVPI proof coefficient", i)
							}
						}
						resolver := &SMTConflictResolver{state: state}
						proof, err := resolver.conflictProof(conflict, SMTNullLiteral, nil)
						astTestMust(t, err)
						if proof.IsNull() {
							t.Fatal("UTVPI conflict did not produce a proof")
						}
					}
					astTestMust(t, state.PopScopes(1))
					if state.inconsistent || len(theory.atoms) != 0 || len(theory.assertions) != 0 || theory.NumVariables() != 1 || len(theory.graph.graph.edges) != 0 {
						t.Fatal("child UTVPI state survived pop")
					}
					status, err := theory.FinalCheck()
					astTestMust(t, err)
					if status != SMTFinalCheckDone {
						t.Fatal("empty parent failed after pop", status)
					}
					parentStats := statistics.New()
					astTestMust(t, theory.CollectStatistics(parentStats))
					parentCounts := map[statistics.Key]uint32{}
					for i := 0; i < parentStats.Size(); i++ {
						parentCounts[parentStats.Key(i)] = parentStats.UintValue(i)
					}
					if !reflect.DeepEqual(counts, parentCounts) {
						t.Fatal("cumulative UTVPI statistics were incorrectly trailed", counts, parentCounts)
					}
				})
			}
		}
	}
}

func TestSMTUTVPIContextShortcutConflictProof(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTUTVPITheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	sort, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	variables := []ast.ASTHandle{}
	for i := 0; i < 5; i++ {
		x, err := m.MkConst(astTestSymbol(t, m, fmt.Sprint("utvpi-shortcut-", i)), sort)
		astTestMust(t, err)
		astTestMust(t, state.Internalize(x, false))
		variables = append(variables, x)
	}
	assert := func(source, target int, weight int64) SMTLiteral {
		difference, err := m.MkFamilyApp(family, smtArithOpSub, nil, []ast.ASTHandle{variables[target], variables[source]})
		astTestMust(t, err)
		n, err := m.ArithmeticNumeralRat(big.NewRat(weight, 1), true)
		astTestMust(t, err)
		formula, err := m.MkFamilyApp(family, smtArithOpLE, nil, []ast.ASTHandle{difference, n})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(formula, true))
		literal, err := state.Literal(formula)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
		astTestMust(t, state.PropagateTheoryAssignments())
		astTestMust(t, theory.Propagate())
		return literal
	}
	for i := 0; i < 3; i++ {
		assert(i, i+1, 0)
	}
	lastChain := assert(3, 4, 0)
	assert(1, 3, 1)
	full := assert(0, 3, 2)
	closing := assert(4, 0, -3)
	original, err := theory.graph.graph.NegativeCycleEdges()
	astTestMust(t, err)
	if len(original) != 5 {
		t.Fatal("missing original five-edge parent cycle", original)
	}
	conflict, found := state.Conflict()
	if !found {
		t.Fatal("missing shortcut arithmetic conflict")
	}
	owner, _ := conflict.Justification()
	extended, ok := owner.(*SMTExtTheoryConflictJustification)
	if !ok {
		t.Fatal("missing extended UTVPI conflict")
	}
	if !reflect.DeepEqual(extended.Literals, []SMTLiteral{closing, lastChain, full}) {
		t.Fatal("conflict did not use the shorter source antecedents", extended.Literals)
	}
	parameters := extended.Parameters()
	if len(parameters) != 4 {
		t.Fatal("shortened proof retained obsolete parent-edge coefficients", len(parameters))
	}
	for _, parameter := range parameters[1:] {
		want, err := m.NewParameterRat(big.NewRat(1, 1))
		astTestMust(t, err)
		equal, err := m.ParameterEqual(parameter, want)
		astTestMust(t, err)
		astTestMust(t, m.DelParameter(&want))
		if !equal {
			t.Fatal("binary shortcut lost coefficient one")
		}
	}
	resolver := &SMTConflictResolver{state: state}
	proof, err := resolver.conflictProof(conflict, SMTNullLiteral, nil)
	astTestMust(t, err)
	if proof.IsNull() {
		t.Fatal("shortened conflict did not build a proof")
	}
}

func TestSMTUTVPIMixedBridgeAndSumModelRefusal(t *testing.T) {
	for _, bridge := range []bool{false, true} {
		t.Run(fmt.Sprint(bridge), func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			params, err := cfg.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
			name := "auto_config"
			astTestMust(t, cfg.Context.SetBoolName(&params, &name, false))
			name = "arith.solver"
			astTestMust(t, cfg.Context.SetUIntName(&params, &name, 4))
			solver, err := NewSMTSolver(m, &params, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			astTestMust(t, solver.SetProduceModels(true))
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			intSort, err := m.MkFamilySort(family, 1, nil)
			astTestMust(t, err)
			realSort, err := m.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			i, err := m.MkConst(astTestSymbol(t, m, "utvpi-mixed-i"), intSort)
			astTestMust(t, err)
			r, err := m.MkConst(astTestSymbol(t, m, "utvpi-mixed-r"), realSort)
			astTestMust(t, err)
			root := func(e ast.ASTHandle) {
				ref, err := m.NewRef(e)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
			}
			root(i)
			root(r)
			app := func(op int32, args ...ast.ASTHandle) ast.ASTHandle {
				e, err := m.MkFamilyApp(family, op, nil, args)
				astTestMust(t, err)
				root(e)
				return e
			}
			bound := func(op int32, expression ast.ASTHandle, value *big.Rat, integer bool) {
				n, err := m.ArithmeticNumeralRat(value, integer)
				astTestMust(t, err)
				astTestMust(t, solver.Assert(app(op, expression, n)))
			}
			bound(smtArithOpGE, i, big.NewRat(0, 1), true)
			bound(smtArithOpLE, i, big.NewRat(1, 1), true)
			converted := app(smtArithOpToReal, i)
			expression := r
			if !bridge {
				expression = app(smtArithOpAdd, converted, r)
			}
			bound(smtArithOpGT, expression, big.NewRat(0, 1), false)
			bound(smtArithOpLT, expression, big.NewRat(1, 3), false)
			if bridge {
				eq, err := m.MkEq(r, converted)
				astTestMust(t, err)
				astTestMust(t, solver.Assert(eq))
			}
			status, err := solver.CheckSat(nil)
			var source z3exception.Z3Exception
			if status != lbool.Undef || !errors.As(err, &source) || source.Message() != "invalid rational value passed as an integer" {
				t.Fatal("mixed source model refusal", status, err)
			}
		})
	}
}

func TestSMTUTVPIEqualityEventsAndScopedFragmentRefusal(t *testing.T) {
	for _, integer := range []bool{false, true} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			theory, err := NewSMTUTVPITheory(family)
			astTestMust(t, err)
			astTestMust(t, state.RegisterTheory(theory))
			zero, err := m.ArithmeticNumeralRat(new(big.Rat), integer)
			astTestMust(t, err)
			sort, err := m.GetSort(zero)
			astTestMust(t, err)
			variables := []ast.ASTHandle{}
			for _, name := range []string{"utvpi-event-x", "utvpi-event-y", "utvpi-event-z"} {
				v, err := m.MkConst(astTestSymbol(t, m, name), sort)
				astTestMust(t, err)
				astTestMust(t, state.Internalize(v, false))
				variables = append(variables, v)
			}
			left := state.context.enodes.get(variables[0]).TheoryVar(family)
			right := state.context.enodes.get(variables[1]).TheoryVar(family)
			astTestMust(t, state.PushScope())
			astTestMust(t, state.enqueueTheoryEquality(family, left, right))
			astTestMust(t, state.PropagateTheoryEvents())
			if theory.adapter.NumAxioms() != 1 || len(theory.atoms) != 2 {
				t.Fatal("active UTVPI equality callback did not create paired bounds")
			}
			astTestMust(t, theory.NewDisequality(left, right))
			if theory.adapter.NumAxioms() != 1 {
				t.Fatal("duplicate equality pair was not suppressed")
			}
			stats := statistics.New()
			astTestMust(t, theory.CollectStatistics(stats))
			counts := map[statistics.Key]uint32{}
			for i := 0; i < stats.Size(); i++ {
				counts[stats.Key(i)] = stats.UintValue(i)
			}
			if counts["core->utvpi eqs"] != 1 || counts["core->utvpi diseqs"] != 0 || counts["arith eq adapter"] != 1 {
				t.Fatal("active source equality/disequality callback counters", counts)
			}
			astTestMust(t, state.PopScopes(1))
			if len(theory.atoms) != 0 || len(theory.graph.graph.edges) != 0 {
				t.Fatal("UTVPI equality adapter products survived pop")
			}
			astTestMust(t, state.PushScope())
			astTestMust(t, state.enqueueTheoryDisequality(family, left, right))
			astTestMust(t, state.PropagateTheoryEvents())
			if theory.adapter.NumAxioms() != 2 || len(theory.atoms) != 2 {
				t.Fatal("popped pair was not recreated")
			}
			astTestMust(t, state.PopScopes(1))
			astTestMust(t, state.PushScope())
			sum, err := m.MkFamilyApp(family, smtArithOpAdd, nil, variables)
			astTestMust(t, err)
			formula, err := m.MkFamilyApp(family, smtArithOpLE, nil, []ast.ASTHandle{sum, zero})
			astTestMust(t, err)
			handled, err := theory.InternalizeAtom(formula, true)
			astTestMust(t, err)
			if handled {
				t.Fatal("UTVPI accepted three nonzero variables")
			}
			status, err := theory.FinalCheck()
			astTestMust(t, err)
			if status != SMTFinalCheckGiveUp {
				t.Fatal("unsupported fragment did not give up", status)
			}
			astTestMust(t, state.PopScopes(1))
			status, err = theory.FinalCheck()
			astTestMust(t, err)
			if status != SMTFinalCheckDone || theory.unsupported {
				t.Fatal("child fragment refusal poisoned its parent", status)
			}
		})
	}
}
