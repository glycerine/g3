package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtPBSimplexFixture(t *testing.T) (*ast.ASTManager, *SMTInternalizerState, *SMTPseudoBooleanTheory, []ast.ASTHandle) {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterPseudoBooleanPlugin()
	astTestMust(t, err)
	_, _, s := smtInternalizerFixture(t, m, nil)
	pb, err := NewSMTPseudoBooleanTheory(family)
	astTestMust(t, err)
	pb.controls.EnableSimplex = true
	astTestMust(t, s.RegisterTheory(pb))
	var inputs []ast.ASTHandle
	for _, name := range []string{"simplex-x", "simplex-y", "simplex-z"} {
		h := smtRelevancyBool(t, m, name)
		inputs = append(inputs, h)
		astTestMust(t, s.Internalize(h, true))
	}
	return m, s, pb, inputs
}

func TestSMTPBSimplexFreshTranslationAndAuxiliaryTableaux(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	params, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
	name := "pb.enable_simplex"
	astTestMust(t, cfg.Context.SetBoolName(&params, &name, true))
	s, err := NewSMTSolver(m, &params, "QF_FD", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	family, err := m.RegisterPseudoBooleanPlugin()
	astTestMust(t, err)
	var xs []ast.ASTHandle
	for _, name := range []string{"fresh-simplex-x", "fresh-simplex-y", "fresh-simplex-z"} {
		xs = append(xs, smtRelevancyBool(t, m, name))
	}
	h := smtPBResolveExpression(t, m, family, xs, []int32{4, 3, 2}, 6)
	astTestMust(t, s.Assert(h))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("source weighted solver", status)
	}
	original := s.kernel.internalizer.theories[family].(*SMTPseudoBooleanTheory)
	if original.simplex == nil || len(original.simplex.rows) == 0 {
		t.Fatal("source never installed a simplex tableau")
	}
	name = "pb.enable_simplex"
	astTestMust(t, cfg.Context.SetBoolName(&params, &name, false))
	astTestMust(t, s.UpdateParams(&params))
	translated, err := s.Translate(m, nil)
	astTestMust(t, err)
	copy := translated.(*SMTSolver)
	t.Cleanup(func() { astTestMust(t, copy.Close()) })
	fresh := copy.kernel.internalizer.theories[family].(*SMTPseudoBooleanTheory)
	if !fresh.controls.EnableSimplex || fresh.simplex != nil {
		t.Fatal("translation copied mutable tableau or newer driver flag")
	}
	status, err = copy.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || fresh.simplex == nil || fresh.simplex.solver == original.simplex.solver {
		t.Fatal("translated solver did not build an independent tableau")
	}
	checker := s.kernel.internalizer.quantifierManager.modelChecker
	astTestMust(t, checker.initAuxiliaryContext())
	auxState := checker.auxiliaryContext.kernel.internalizer
	aux := auxState.theories[family].(*SMTPseudoBooleanTheory)
	if !aux.controls.EnableSimplex || aux.simplex != nil {
		t.Fatal("auxiliary copied live tableau or lost installed control snapshot")
	}
	astTestMust(t, auxState.Internalize(h, true))
	if aux.simplex == nil || aux.simplex.solver == original.simplex.solver || aux.simplex.solver == fresh.simplex.solver {
		t.Fatal("auxiliary simplex workspace is shared")
	}
}

func smtPBSimplexAtom(t *testing.T, s *SMTInternalizerState, pb *SMTPseudoBooleanTheory, args []ast.ASTHandle, coefficients []int32, bound int32, equality bool) SMTLiteral {
	t.Helper()
	var parameters []ast.ASTParameter
	for _, v := range append([]int32{bound}, coefficients...) {
		p, err := s.manager.NewParameterInt(v)
		astTestMust(t, err)
		parameters = append(parameters, p)
	}
	kind := int32(ast.ASTPBOpGE)
	if equality {
		kind = ast.ASTPBOpEQ
	}
	h, err := s.manager.MkFamilyApp(pb.FamilyID(), kind, parameters, args)
	astTestMust(t, err)
	for i := range parameters {
		astTestMust(t, s.manager.DelParameter(&parameters[i]))
	}
	astTestMust(t, s.Internalize(h, true))
	l, err := s.Literal(h)
	astTestMust(t, err)
	return l
}

func TestSMTPBSimplexSharedRowsBoundConflictsAndRollback(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			m, s, pb, xs := smtPBSimplexFixture(t)
			if proofs {
				astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			}
			a := smtPBSimplexAtom(t, s, pb, xs, []int32{1, 1, 1}, 2, false)
			b := smtPBSimplexAtom(t, s, pb, xs, []int32{1, 1, 1}, 3, false)
			rowA, rowB := pb.simplex.rows[a.Var()], pb.simplex.rows[b.Var()]
			if rowA.slack != rowB.slack || len(pb.simplex.solver.BasicRows()) != 1 {
				t.Fatal("same coefficients did not share source representative", rowA, rowB)
			}
			for repeat := 0; repeat < 2; repeat++ {
				astTestMust(t, s.PushScope())
				for _, l := range []SMTLiteral{a.Negated(), b} {
					smtPBResolutionRoot(t, s, l, proofs)
					s.booleanData[l.Var()].SetAssumption(true)
				}
				_, err := s.PropagateCore()
				astTestMust(t, err)
				conflict, found := s.Conflict()
				if !found {
					t.Fatal("inconsistent shared-slack bounds were accepted")
				}
				resolver := smtCoreWalkResolver(t, s)
				core, err := resolver.UnsatCore(conflict, s.ConflictLiteral())
				astTestMust(t, err)
				seen := make(map[SMTLiteral]bool)
				for _, l := range core {
					if l != a.Negated() && l != b {
						t.Fatal("bound conflict invented a premise", l)
					}
					seen[l] = true
				}
				if len(seen) != 2 {
					t.Fatal("bound conflict lost an assumption", core)
				}
				if proofs {
					proof, err := resolver.conflictProof(conflict, s.ConflictLiteral(), nil)
					astTestMust(t, err)
					fact, err := m.GetProofFact(proof)
					astTestMust(t, err)
					if !m.IsFalse(fact) {
						t.Fatal("shared-bound proof is not false")
					}
				}
				astTestMust(t, s.PopScopes(1))
				if _, valid := pbBound(t, pb.simplex.solver, rowA.slack, true); valid {
					t.Fatal("predicate lower bound survived pop")
				}
				if _, valid := pbBound(t, pb.simplex.solver, rowA.slack, false); valid {
					t.Fatal("predicate upper bound survived pop")
				}
			}
		})
	}
}

func TestSMTPBSimplexInfeasibleRowExplainsOnlyAssignedPredicates(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			m, s, pb, xs := smtPBSimplexFixture(t)
			if proofs {
				astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			}
			var atoms []SMTLiteral
			for _, pair := range [][2]int{{0, 1}, {1, 2}, {0, 2}} {
				atoms = append(atoms, smtPBSimplexAtom(t, s, pb, []ast.ASTHandle{xs[pair[0]], xs[pair[1]]}, []int32{1, 1}, 1, false))
			}
			var negated []ast.ASTHandle
			for _, x := range xs {
				h, err := m.MkNot(x)
				astTestMust(t, err)
				negated = append(negated, h)
			}
			atoms = append(atoms, smtPBSimplexAtom(t, s, pb, negated, []int32{1, 1, 1}, 2, false))
			astTestMust(t, s.PushScope())
			for _, a := range atoms {
				smtPBResolutionRoot(t, s, a, proofs)
				s.booleanData[a.Var()].SetAssumption(true)
			}
			_, err := s.PropagateCore()
			astTestMust(t, err)
			conflict, found := s.Conflict()
			if !found {
				t.Fatal("inconsistent fractional relaxation accepted")
			}
			if pb.simplex.solver.Stats.NumInfeasible == 0 || pb.statistics.conflicts != 1 {
				t.Fatal("native watches replaced simplex infeasible-row path", pb.simplex.solver.Stats, pb.statistics)
			}
			resolver := smtCoreWalkResolver(t, s)
			core, err := resolver.UnsatCore(conflict, s.ConflictLiteral())
			astTestMust(t, err)
			seen := make(map[SMTLiteral]bool)
			for _, l := range core {
				seen[l] = true
			}
			if len(seen) != 4 {
				t.Fatal("row conflict lost predicate explanations", core)
			}
			for _, l := range core {
				found := false
				for _, a := range atoms {
					found = found || l == a
				}
				if !found {
					t.Fatal("row conflict invented premise", l)
				}
			}
			if proofs {
				proof, err := resolver.conflictProof(conflict, s.ConflictLiteral(), nil)
				astTestMust(t, err)
				fact, err := m.GetProofFact(proof)
				astTestMust(t, err)
				if !m.IsFalse(fact) {
					t.Fatal("row proof is not false")
				}
			}
			astTestMust(t, s.PopScopes(1))
			ok, err := pb.simplexCheckFeasible()
			astTestMust(t, err)
			if !ok {
				t.Fatal("row infeasibility survived scope pop")
			}
		})
	}
}

func TestSMTPBSimplexBoundTrailAndRepresentativeRemoval(t *testing.T) {
	_, s, pb, xs := smtPBSimplexFixture(t)
	for repeat := 0; repeat < 2; repeat++ {
		astTestMust(t, s.PushScope())
		a := smtPBSimplexAtom(t, s, pb, xs, []int32{1, 1, 1}, 2, false)
		_ = smtPBSimplexAtom(t, s, pb, xs, []int32{1, 1, 1}, 3, false)
		variable := pb.simplex.rows[a.Var()].slack
		astTestMust(t, s.PushScope())
		bound := testRI(big.NewRat(1, 1))
		ok, err := pb.simplexUpdateBound(variable, a, true, &bound)
		astTestMust(t, err)
		if !ok {
			t.Fatal("initial bound rejected")
		}
		astTestMust(t, s.PushScope())
		setSmall(&bound.Main, 2)
		ok, err = pb.simplexUpdateBound(variable, a, true, &bound)
		astTestMust(t, err)
		if !ok {
			t.Fatal("stronger bound rejected")
		}
		astTestMust(t, s.PopScopes(1))
		got, valid := pbBound(t, pb.simplex.solver, variable, true)
		if !valid || got.Main.Cmp(big.NewRat(1, 1)) != 0 || smtPBExplain(pb.simplex.lower, variable) != a {
			t.Fatal("bound/explanation restoration", got, valid)
		}
		astTestMust(t, s.PopScopes(1))
		if _, valid = pbBound(t, pb.simplex.solver, variable, true); valid {
			t.Fatal("new bound survived rollback")
		}
		astTestMust(t, s.PopScopes(1))
		if len(pb.simplex.rows) != 0 || len(pb.simplex.representatives) != 0 || len(pb.simplex.variables) != 0 || len(pb.simplex.solver.BasicRows()) != 0 {
			t.Fatal("representative/alias row lifetime mismatch")
		}
	}
}

func TestSMTPBSimplexNestedPredicateKeepsBooleanAndSlackDistinct(t *testing.T) {
	_, s, pb, xs := smtPBSimplexFixture(t)
	a := smtPBSimplexAtom(t, s, pb, xs[:2], []int32{1, 1}, 2, false)
	aExpression, err := s.BoolExpression(a.Var())
	astTestMust(t, err)
	b := smtPBSimplexAtom(t, s, pb, []ast.ASTHandle{aExpression, xs[2]}, []int32{1, 1}, 1, true)
	z, err := s.Literal(xs[2])
	astTestMust(t, err)
	for _, l := range []SMTLiteral{a, b, z.Negated()} {
		smtPBWatchAssign(t, s, l)
	}
	if s.Inconsistent() {
		t.Fatal("satisfiable nested PB confused parent Boolean value one with row sum two")
	}
	ok, err := pb.simplexCheckFeasible()
	astTestMust(t, err)
	if !ok {
		t.Fatal("nested PB relaxation is infeasible")
	}
}

func TestSMTPBSimplexPivotedScopedRowRemovalPreservesOuterRows(t *testing.T) {
	_, s, pb, xs := smtPBSimplexFixture(t)
	outer := smtPBSimplexAtom(t, s, pb, xs[:2], []int32{1, 1}, 1, false)
	smtPBWatchAssign(t, s, outer)
	for repeat := 0; repeat < 2; repeat++ {
		before := pb.simplex.solver.Stats.NumPivots
		astTestMust(t, s.PushScope())
		_ = smtPBSimplexAtom(t, s, pb, xs[1:], []int32{1, 1}, 1, false)
		stronger := smtPBSimplexAtom(t, s, pb, xs[1:], []int32{1, 1}, 2, false)
		smtPBWatchAssign(t, s, stronger)
		// Source trails bounds, not numeric values or pivots. On the second
		// pass the retained feasible values may already satisfy the new row.
		if s.Inconsistent() || repeat == 0 && pb.simplex.solver.Stats.NumPivots <= before {
			t.Fatal("scoped row did not exercise a feasible pivot")
		}
		astTestMust(t, s.PopScopes(1))
		if len(pb.simplex.rows) != 1 || len(pb.simplex.representatives) != 1 || len(pb.simplex.solver.BasicRows()) != 1 {
			t.Fatal("pivoted scoped row or alias survived pop")
		}
		if _, found := pb.simplex.rows[outer.Var()]; !found {
			t.Fatal("outer row metadata was removed")
		}
		ok, err := pb.simplexCheckFeasible()
		astTestMust(t, err)
		if !ok {
			t.Fatal("pivoted row deletion broke the retained outer equation")
		}
		info := pb.simplex.rows[outer.Var()]
		slack := pbValue(t, pb.simplex.solver, info.slack)
		x, err := s.Literal(xs[0])
		astTestMust(t, err)
		y, err := s.Literal(xs[1])
		astTestMust(t, err)
		sum := pbValue(t, pb.simplex.solver, uint32(x.Var()))
		other := pbValue(t, pb.simplex.solver, uint32(y.Var()))
		sum.Main.Add(&sum.Main, &other.Main)
		sum.Epsilon.Add(&sum.Epsilon, &other.Epsilon)
		if sum.Cmp(slack) != 0 || slack.Main.Cmp(big.NewRat(1, 1)) < 0 {
			t.Fatal("outer numeric row no longer represents x+y>=1", sum, slack)
		}
	}
}

func TestSMTPBSimplexSignedWeightedValuationOracle(t *testing.T) {
	for _, weights := range [][3]int32{{4, 3, 2}, {1, 1, 1}, {-2, 3, 1}, {3, -2, 4}} {
		for _, bound := range []int32{-1, 0, 1, 3, 6} {
			for _, equality := range []bool{false, true} {
				for mask := 0; mask < 8; mask++ {
					for _, positive := range []bool{false, true} {
						t.Run(fmt.Sprintf("%v/k%d/eq%v/m%d/pos%v", weights, bound, equality, mask, positive), func(t *testing.T) {
							_, s, pb, xs := smtPBSimplexFixture(t)
							a := smtPBSimplexAtom(t, s, pb, xs, weights[:], bound, equality)
							if !positive {
								a = a.Negated()
							}
							astTestMust(t, s.AssignLiteral(a, NewSMTBooleanAxiom()))
							sum := int32(0)
							for i, x := range xs {
								l, err := s.Literal(x)
								astTestMust(t, err)
								if mask&(1<<i) == 0 {
									l = l.Negated()
								} else {
									sum += weights[i]
								}
								astTestMust(t, s.AssignLiteral(l, NewSMTBooleanAxiom()))
							}
							_, err := s.PropagateCore()
							astTestMust(t, err)
							truth := sum >= bound
							if equality {
								truth = sum == bound
							}
							if s.Inconsistent() != (truth != positive) {
								t.Fatal("signed weighted predicate valuation", sum, bound, positive, s.Inconsistent())
							}
						})
					}
				}
			}
		}
	}
}

// pbBound reads a (small) solver bound.
func pbBound(t *testing.T, solver *simplex.Solver, v uint32, lower bool) (*bigInf, bool) {
	t.Helper()
	out := simplex.ZeroInfinitesimal()
	var valid bool
	var err error
	if lower {
		valid, err = solver.Lower(v, &out)
	} else {
		valid, err = solver.Upper(v, &out)
	}
	astTestMust(t, err)
	return asBigInf(&out), valid
}

// pbValue reads a (small) solver value.
func pbValue(t *testing.T, solver *simplex.Solver, v uint32) *bigInf {
	t.Helper()
	out := simplex.ZeroInfinitesimal()
	astTestMust(t, solver.Value(v, &out))
	return asBigInf(&out)
}
