package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
)

func TestSMTArithmeticOwnEqualityRetainedForReplayWithoutContextBounce(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, s := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTMixedLinearArithmeticTheory(family)
	astTestMust(t, err)
	astTestMust(t, theory.ConfigureArithmetic(SMTArithState{EqBounds: true, Reflect: true}))
	astTestMust(t, s.RegisterTheory(theory))
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	sort, err := m.GetSort(zero)
	astTestMust(t, err)
	var variables []SMTTheoryVar
	var nodes []*SMTEnode
	for _, name := range []string{"own-equality-x", "own-equality-y"} {
		h, err := m.MkConst(astTestSymbol(t, m, name), sort)
		astTestMust(t, err)
		astTestMust(t, s.Internalize(h, false))
		node := s.context.enodes.get(h)
		variables = append(variables, node.TheoryVar(family))
		nodes = append(nodes, node)
	}
	for _, deliver := range []bool{false, true} {
		astTestMust(t, s.PushScope())
		before := len(s.ownedJustifications)
		progress, err := theory.propagateEquality(s, theory.simplexVariables[variables[0]], theory.simplexVariables[variables[1]], nil)
		astTestMust(t, err)
		if !progress || nodes[0].root == nodes[1].root || len(s.PendingTheoryEqualities()) != 0 {
			t.Fatal("own equality merged eagerly or bounced back through context")
		}
		if deliver {
			astTestMust(t, s.PropagateTheoryAssignments())
			if nodes[0].root != nodes[1].root || len(s.PendingTheoryEqualities()) != 0 {
				t.Fatal("own equality was not merged at the core phase or bounced back")
			}
			astTestMust(t, theory.replayOwnEqualities())
			if len(theory.equalityBounds) != 1 || len(theory.pendingBounds) != 2 || len(s.ownedJustifications) != before+1 {
				t.Fatal("own equality lost replay bounds or scoped explanation ownership")
			}
		} else if !theory.CanPropagate() || len(theory.equalityBounds) != 0 {
			t.Fatal("undelivered own equality lost its pending replay obligation")
		}
		astTestMust(t, s.PopScopes(1))
		if len(theory.equalityBounds) != 0 || len(theory.pendingBounds) != 0 || len(theory.ownEqualities) != 0 || nodes[0].root == nodes[1].root || len(s.ownedJustifications) != before {
			t.Fatal("own equality replay state escaped scope")
		}
	}
}

func TestSMTArithmeticEqualityEventsQueueNativeBoundsAndProofs(t *testing.T) {
	for _, integer := range []bool{false, true} {
		for _, numeric := range []bool{false, true} {
			for _, proofs := range []bool{false, true} {
				for _, nonlinear := range []bool{false, true} {
					if nonlinear && !numeric {
						continue
					}
					t.Run(fmt.Sprint(integer, "/", numeric, "/", proofs, "/", nonlinear), func(t *testing.T) {
						m := astTestManager(t)
						if proofs {
							astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
						}
						family, err := m.RegisterArithmeticPlugin()
						astTestMust(t, err)
						_, _, state := smtInternalizerFixture(t, m, nil)
						theory, err := NewSMTMixedLinearArithmeticTheory(family)
						astTestMust(t, err)
						astTestMust(t, theory.ConfigureArithmetic(SMTArithState{EqBounds: true, Reflect: true, NLArith: true, NLArithGB: true, NLArithGBThreshold: 512, NLArithRounds: 1024, StrongerLemmas: true}))
						astTestMust(t, state.RegisterTheory(theory))
						three, err := m.ArithmeticNumeralRat(big.NewRat(3, 1), integer)
						astTestMust(t, err)
						sort, err := m.GetSort(three)
						astTestMust(t, err)
						x, err := m.MkConst(astTestSymbol(t, m, "native-equality-x"), sort)
						astTestMust(t, err)
						y := three
						if !numeric {
							y, err = m.MkConst(astTestSymbol(t, m, "native-equality-y"), sort)
							astTestMust(t, err)
						}
						astTestMust(t, state.Internalize(x, false))
						astTestMust(t, state.Internalize(y, false))
						astTestMust(t, state.PushScope())
						equality, err := m.MkEq(x, y)
						astTestMust(t, err)
						// Match the context's core equality entry before the theory
						// callback, so an equality event cannot hide behind adapter axioms.
						astTestMust(t, state.internalizeFormulaCore(equality, true))
						literal, err := state.Literal(equality)
						astTestMust(t, err)
						astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
						before := len(state.boolExpr)
						left, right := state.context.enodes.get(x), state.context.enodes.get(y)
						astTestMust(t, state.addEquality(left, right, NewSMTEqualityEquation(literal)))
						astTestMust(t, state.PropagateTheoryEvents())
						if len(theory.equalityBounds) != 1 || len(theory.pendingBounds) != 2 || len(state.boolExpr) != before || len(theory.atoms) != 0 || theory.equalityAdapter.NumAxioms() != 0 {
							t.Fatal("equality callback did not create direct native endpoints")
						}
						astTestMust(t, theory.Propagate())
						pair := theory.equalityBounds[0]
						column := theory.simplexVariables[pair.lower.variable]
						uses := []simplex.ArithmeticBoundUse{{Column: column, Lower: true, Coefficient: *big.NewRat(2, 3)}, {Column: column, Lower: false, Coefficient: *big.NewRat(-5, 7)}}
						antecedents, known, err := smtArithmeticExplainBounds(theory.solver, uses, proofs, theory.equalityBounds)
						astTestMust(t, err)
						if !known || len(antecedents.literals) != 0 || len(antecedents.equalities) != 2 {
							t.Fatal("replay lost the equality-premise variant", antecedents)
						}
						if proofs && (len(antecedents.equalityCoefficients) != 2 || antecedents.equalityCoefficients[0].Cmp(big.NewRat(2, 3)) != 0 || antecedents.equalityCoefficients[1].Cmp(big.NewRat(-5, 7)) != 0) {
							t.Fatal("equality Farkas coefficients changed")
						}
						term, limit := x, big.NewRat(2, 1)
						if !numeric {
							negative, err := m.ArithmeticNumeralRat(big.NewRat(-1, 1), integer)
							astTestMust(t, err)
							negY, err := m.MkFamilyApp(family, smtArithOpMul, nil, []ast.ASTHandle{negative, y})
							astTestMust(t, err)
							term, err = m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{x, negY})
							astTestMust(t, err)
							limit = big.NewRat(-1, 1)
						}
						if nonlinear {
							term, err = m.MkFamilyApp(family, smtArithOpMul, nil, []ast.ASTHandle{x, x})
							astTestMust(t, err)
							limit = big.NewRat(8, 1)
						}
						bound, err := m.ArithmeticNumeralRat(limit, integer)
						astTestMust(t, err)
						constraint, err := m.MkFamilyApp(family, smtArithOpLE, nil, []ast.ASTHandle{term, bound})
						astTestMust(t, err)
						astTestMust(t, state.Internalize(constraint, true))
						boundLiteral, err := state.Literal(constraint)
						astTestMust(t, err)
						astTestMust(t, state.AssignLiteral(boundLiteral, NewSMTBooleanAxiom()))
						astTestMust(t, state.PropagateTheoryAssignments())
						astTestMust(t, theory.Propagate())
						conflict, found := state.Conflict()
						if !found {
							t.Fatal("native equality endpoints did not conflict with an incompatible bound")
						}
						owner, ok := conflict.Justification()
						if !ok {
							t.Fatal("missing arithmetic conflict owner")
						}
						extended, ok := owner.(*SMTExtTheoryConflictJustification)
						if !ok || len(extended.Equalities) == 0 {
							t.Fatal("arithmetic conflict lowered away its equality premises")
						}
						if proofs && !nonlinear && len(extended.Parameters()) < 3 {
							t.Fatal("arithmetic conflict lost its numerical equality coefficients")
						}
						if proofs {
							resolver := &SMTConflictResolver{state: state}
							proof, err := resolver.conflictProof(conflict, SMTNullLiteral, nil)
							astTestMust(t, err)
							fact, err := m.GetProofFact(proof)
							astTestMust(t, err)
							if !m.IsFalse(fact) {
								t.Fatal("native equality conflict proof did not establish false")
							}
						}
						premises, err := NewSMTAntecedentExpander().ExpandJustification(owner)
						astTestMust(t, err)
						seen := make(map[SMTLiteral]bool)
						for _, premise := range premises {
							seen[premise] = true
						}
						if !seen[literal] || !seen[boundLiteral] {
							t.Fatal("conflict lost equality or arithmetic provenance", premises)
						}
						astTestMust(t, state.PopScopes(1))
						if len(extended.Parameters()) != 0 {
							t.Fatal("arithmetic equality-conflict parameters survived backtracking")
						}
						if len(theory.equalityBounds) != 0 || len(theory.pendingBounds) != 0 || len(theory.derivedBounds) != 0 || state.inconsistent || left.root == right.root {
							t.Fatal("equality bounds survived their logical scope")
						}
					})
				}
			}
		}
	}
}

func TestSMTArithmeticMixedPremiseAccumulation(t *testing.T) {
	solver := simplex.NewSolver(nil)
	dependencies := []simplex.BoundDependency{{Literal: 12, Coefficient: *big.NewRat(2, 3)}, {Equality: 1, Coefficient: *big.NewRat(3, 5)}, {Equality: 2, Coefficient: *big.NewRat(5, 7)}}
	astTestMust(t, solver.SetLowerWithDependencies(1, simplex.RationalInfinitesimal(big.NewRat(0, 1)), dependencies))
	astTestMust(t, solver.SetLowerWithDependencies(2, simplex.RationalInfinitesimal(big.NewRat(0, 1)), dependencies[1:2]))
	uses := []simplex.ArithmeticBoundUse{{Column: 1, Lower: true, Coefficient: *big.NewRat(7, 2)}, {Column: 2, Lower: true, Coefficient: *big.NewRat(-1, 2)}}
	weighted := smtArithmeticWeightedBoundExplanation(solver, uses)
	if weighted == nil || len(weighted.dependencies) != 3 || weighted.dependencies[1].Equality != 1 || weighted.dependencies[2].Equality != 2 || weighted.dependencies[1].Coefficient.Cmp(big.NewRat(9, 5)) != 0 {
		t.Fatal("literal/equality premise variants collided or lost weights", weighted)
	}
	plain := smtArithmeticPlainBoundExplanation(solver, uses)
	if plain == nil || len(plain.dependencies) != 3 || plain.dependencies[1].Equality != 1 || plain.dependencies[2].Equality != 2 {
		t.Fatal("plain bound accumulator lost distinct equality premises", plain)
	}
	for _, dependency := range plain.dependencies {
		if dependency.Coefficient.Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal("plain derived bound retained numerical multipliers")
		}
	}
}
