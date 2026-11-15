package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/lp"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSMTLRALPDerivedBoundStrictnessReasonsAndScopes(t *testing.T) {
	for _, strict := range []bool{false, true} {
		for _, small := range []uint32{0, 16} {
			for _, proofs := range []bool{false, true} {
				t.Run(fmt.Sprint(strict, small, proofs), func(t *testing.T) {
					f := newLRAFrontendFixture(t, true)
					if proofs {
						astTestMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
					}
					config := f.a.config
					config.SmallLemmaSize = small
					astTestMust(t, f.a.ConfigureArithmetic(config))
					x, y := f.variable("lra-row-x"), f.variable("lra-row-y")
					op := smtArithOpGE
					if strict {
						op = smtArithOpLE
					}
					target := f.app(op, x, f.number(3))
					astTestMust(t, f.s.Internalize(target, true))
					astTestMust(t, f.a.Propagate())
					astTestMust(t, f.s.PushScope())
					sum := f.app(smtArithOpAdd, x, y)
					premise := f.app(op, sum, f.number(5))
					other := f.app(smtArithOpLE, y, f.number(2))
					f.assert(premise, !strict)
					f.assert(other, true)
					f.propagate(true)
					literal, err := f.s.Literal(target)
					astTestMust(t, err)
					if strict {
						literal = literal.Negated()
					}
					value, err := f.s.Assignment(literal)
					astTestMust(t, err)
					if value != lbool.True || f.a.lpPropagations == 0 {
						t.Fatal("LP row did not propagate", value, f.a.lpPropagations)
					}
					left, err := f.s.Literal(premise)
					astTestMust(t, err)
					if strict {
						left = left.Negated()
					}
					right, err := f.s.Literal(other)
					astTestMust(t, err)
					want := boundClauseSet([]SMTLiteral{left.Negated(), right.Negated(), literal})
					if small != 0 {
						found := false
						for _, c := range f.s.clauses {
							if c.clause != nil && c.clause.IsAuxLemma() && boundClauseSet(c.Literals) == want {
								found = true
								if proofs {
									builder := &smtConflictProofBuilder{state: f.s, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle), objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle)}
									proof, err := builder.clauseProof(c.clause)
									astTestMust(t, err)
									ref, err := f.m.NewRef(proof)
									astTestMust(t, err)
									fact, err := f.m.GetProofFact(proof)
									astTestMust(t, err)
									kind, err := f.m.GetDeclKind(fact)
									astTestMust(t, err)
									if kind != ast.ASTOpOr {
										t.Fatal("LP lemma proof lost clause fact", kind)
									}
									astTestMust(t, f.m.DelRef(&ref))
								}
							}
						}
						if !found {
							t.Fatal("LP implication lost its two signed original bound witnesses", want)
						}
					} else {
						found := false
						for _, owner := range f.s.ownedJustifications {
							if j, ok := owner.(*SMTExtTheoryPropagationJustification); ok && j.Consequent == literal {
								if len(j.Literals) != 2 || boundClauseSet(j.Literals) != boundClauseSet([]SMTLiteral{left, right}) || len(j.Parameters()) != 0 {
									t.Fatal("LP external reason", j.Literals, j.Parameters())
								}
								found = true
							}
						}
						if !found {
							t.Fatal("missing LP external propagation reason")
						}
					}
					if f.a.propagator != nil {
						t.Fatal("successful propagator remained owned")
					}
					astTestMust(t, f.s.PopScopes(1))
					value, err = f.s.Assignment(literal)
					astTestMust(t, err)
					if value != lbool.Undef {
						t.Fatal("LP implication assignment survived scope", value)
					}
					if len(f.a.boundChecks) != 0 {
						t.Fatal("pending bound checks survived pop")
					}
					f.propagate(true)
				})
			}
		}
	}
}

func TestSMTLRAImpliedBoundTruthTable(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	for _, kind := range []lp.LConstraintKind{lp.LConstraintLE, lp.LConstraintLT, lp.LConstraintGE, lp.LConstraintGT} {
		for _, upper := range []bool{false, true} {
			for _, threshold := range []int64{-1, 0, 1} {
				b := &smtLRABound{boolean: 17, upper: upper, value: *big.NewRat(threshold, 1)}
				got := f.a.impliedBoundLiteral(kind, new(big.Rat), b)
				// Enumerate rational/epsilon witnesses on both sides of zero.
				// The target is constant on this set exactly when it is implied.
				seenTrue, seenFalse := false, false
				for whole := -4; whole <= 4; whole++ {
					for epsilon := -1; epsilon <= 1; epsilon++ {
						cmp := whole
						if cmp == 0 {
							cmp = epsilon
						}
						allowed := kind == lp.LConstraintLE && cmp <= 0 || kind == lp.LConstraintLT && cmp < 0 || kind == lp.LConstraintGE && cmp >= 0 || kind == lp.LConstraintGT && cmp > 0
						if !allowed {
							continue
						}
						comparison := int64(whole) - 2*threshold
						if comparison == 0 {
							comparison = int64(epsilon)
						}
						truth := comparison >= 0
						if upper {
							truth = comparison <= 0
						}
						if truth {
							seenTrue = true
						} else {
							seenFalse = true
						}
					}
				}
				want := SMTNullLiteral
				if seenTrue && !seenFalse {
					want = NewSMTLiteral(17, false)
				}
				if seenFalse && !seenTrue {
					want = NewSMTLiteral(17, true)
				}
				if got != want {
					t.Fatal(kind, upper, threshold, got, "want", want)
				}
			}
		}
	}
}

func TestSMTLRABasicBoundSourceTautologyAndScope(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("lra-basic-x")
	lower := f.app(smtArithOpGE, x, f.number(1))
	strong := f.app(smtArithOpGE, x, f.number(2))
	astTestMust(t, f.s.Internalize(lower, true))
	astTestMust(t, f.s.Internalize(strong, true))
	// Exercise the native basic helper itself, before bound axioms or LP
	// propagation can supply a different reason for this conclusion.
	v1, _ := f.s.BoolVar(lower)
	v2, _ := f.s.BoolVar(strong)
	astTestMust(t, f.s.PushScope())
	before := len(f.s.Clauses())
	astTestMust(t, f.a.propagateBasicBound(f.a.boundByBoolean[v2], true))
	value, err := f.s.Assignment(NewSMTLiteral(v1, false))
	astTestMust(t, err)
	if value != lbool.Undef || len(f.s.Clauses()) != before || f.a.basicPropagations != 1 {
		t.Fatal("native self-premise was silently corrected", value, len(f.s.Clauses()), before)
	}
	xv := f.a.boundByBoolean[v1].variable
	if f.a.unassigned[xv] != 1 {
		t.Fatal("source basic counter not updated")
	}
	astTestMust(t, f.s.PopScopes(1))
	if f.a.unassigned[xv] != 2 {
		t.Fatal("basic counter was not restored")
	}
}

func TestSMTLRALPDerivedCompoundTerm(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("lra-compound-x"), f.variable("lra-compound-y")
	term := f.app(smtArithOpAdd, x, y, f.number(7))
	target := f.app(smtArithOpGE, term, f.number(10))
	astTestMust(t, f.s.Internalize(target, true))
	f.assert(f.app(smtArithOpGE, x, f.number(1)), true)
	f.assert(f.app(smtArithOpGE, y, f.number(2)), true)
	f.propagate(true)
	lit, err := f.s.Literal(target)
	astTestMust(t, err)
	value, err := f.s.Assignment(lit)
	astTestMust(t, err)
	if value != lbool.True || f.a.lpPropagations == 0 {
		t.Fatal("constant-shifted term bound not propagated", value)
	}
}

// Stop just before the source LP callback to exercise that call's owned
// temporaries separately from registration and make-feasible allocation.
func prepareLRALPPropagation(t *testing.T, f *lraFrontendFixture) ast.ASTHandle {
	return prepareLRALPPropagationScale(t, f, big.NewRat(1, 1))
}

func prepareLRALPPropagationScale(t *testing.T, f *lraFrontendFixture, scale *big.Rat) ast.ASTHandle {
	t.Helper()
	x, y := f.variable("lra-prepare-x"), f.variable("lra-prepare-y")
	target := f.app(smtArithOpGE, x, f.number(3))
	astTestMust(t, f.s.Internalize(target, true))
	coefficient := f.pin(f.m.ArithmeticNumeralRat(scale, false))
	five := f.pin(f.m.ArithmeticNumeralRat(new(big.Rat).Mul(scale, big.NewRat(5, 1)), false))
	two := f.pin(f.m.ArithmeticNumeralRat(new(big.Rat).Mul(scale, big.NewRat(2, 1)), false))
	f.assert(f.app(smtArithOpGE, f.app(smtArithOpAdd, f.app(smtArithOpMul, coefficient, x), y), five), true)
	f.assert(f.app(smtArithOpLE, y, two), true)
	astTestMust(t, f.s.PropagateTheoryAssignments())
	for f.a.assignmentHead < len(f.a.assignments) {
		a := f.a.assignments[f.a.assignmentHead]
		b := f.a.boundByBoolean[a.boolean]
		_, err := f.a.numeric.assertBound(b.variable, a.boolean, &b.value, b.upper, a.truth)
		astTestMust(t, err)
		f.a.assignmentHead++
	}
	status, err := f.a.numeric.makeFeasible()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("propagation fixture not feasible", status)
	}
	return target
}

func TestSMTLRALPPropagationCancelRetainsRowsAndResumes(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	target := prepareLRALPPropagation(t, f)
	if len(f.a.numeric.solver.RowsWithChangedBounds.Indices()) == 0 {
		t.Fatal("no touched rows")
	}
	f.m.Limit.Cancel()
	astTestMust(t, f.a.propagateLPBounds())
	f.m.Limit.ResetCancel()
	if f.a.propagator != nil || len(f.a.numeric.solver.RowsWithChangedBounds.Indices()) == 0 {
		t.Fatal("cancellation lost rows or retained temporary")
	}
	lit, err := f.s.Literal(target)
	astTestMust(t, err)
	value, err := f.s.Assignment(lit)
	astTestMust(t, err)
	if value != lbool.Undef {
		t.Fatal("canceled callback assigned a result")
	}
	astTestMust(t, f.a.propagateLPBounds())
	value, err = f.s.Assignment(lit)
	astTestMust(t, err)
	if value != lbool.True || len(f.a.numeric.solver.RowsWithChangedBounds.Indices()) != 0 {
		t.Fatal("resumed callback lost row result", value)
	}
}

func TestSMTLRALPPropagationDisabledRetainsRows(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	target := prepareLRALPPropagation(t, f)
	config := f.a.config
	config.BoundProp = 0
	astTestMust(t, f.a.ConfigureArithmetic(config))
	astTestMust(t, f.a.propagateLPBounds())
	lit, err := f.s.Literal(target)
	astTestMust(t, err)
	value, err := f.s.Assignment(lit)
	astTestMust(t, err)
	if value != lbool.Undef || f.a.lpPropagations != 0 || len(f.a.numeric.solver.RowsWithChangedBounds.Indices()) == 0 {
		t.Fatal("disabled propagation changed state")
	}
	config.BoundProp = 2
	astTestMust(t, f.a.ConfigureArithmetic(config))
	astTestMust(t, f.a.propagateLPBounds())
	value, err = f.s.Assignment(lit)
	astTestMust(t, err)
	if value != lbool.True {
		t.Fatal("reenabled propagation lost pending row")
	}
}

func TestSMTLRAPropagationConflictThresholdSurvivesPop(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	target := prepareLRALPPropagation(t, f)
	config := f.a.config
	config.PropagationThreshold = 1
	astTestMust(t, f.a.ConfigureArithmetic(config))
	astTestMust(t, f.s.PushScope())
	x, err := f.m.GetArg(target, 0)
	astTestMust(t, err)
	f.assert(f.app(smtArithOpGE, x, f.number(0)), false)
	f.propagate(false)
	if f.a.conflictCount != 1 {
		t.Fatal("LRA conflict was not counted", f.a.conflictCount)
	}
	astTestMust(t, f.s.PopScopes(1))
	if f.a.conflictCount != 1 {
		t.Fatal("source conflict count was incorrectly scoped")
	}
	astTestMust(t, f.a.propagateLPBounds())
	lit, err := f.s.Literal(target)
	astTestMust(t, err)
	value, err := f.s.Assignment(lit)
	astTestMust(t, err)
	if value != lbool.Undef || f.a.lpPropagations != 0 {
		t.Fatal("conflict threshold failed to disable LP propagation", value)
	}
	astTestMust(t, f.a.InitializeSearch())
	if f.a.conflictCount != 0 {
		t.Fatal("new source search did not reset conflict threshold")
	}
	astTestMust(t, f.a.propagateLPBounds())
	value, err = f.s.Assignment(lit)
	astTestMust(t, err)
	if value != lbool.True {
		t.Fatal("new search lost touched rows", value)
	}
}

func TestSMTLRAPropagationZeroThresholdDisablesBasicAndLP(t *testing.T) {
	f := newLRAFrontendFixtureWithPropagation(t, true, 2, 0)
	target := prepareLRALPPropagation(t, f)
	x, err := f.m.GetArg(target, 0)
	astTestMust(t, err)
	strong := f.app(smtArithOpGE, x, f.number(4))
	astTestMust(t, f.s.Internalize(strong, true))
	if f.a.numeric.solver.Settings.BoundPropagation {
		t.Fatal("initial LP setting ignored zero propagation threshold")
	}
	astTestMust(t, f.a.propagateLPBounds())
	bv, _ := f.s.BoolVar(strong)
	astTestMust(t, f.a.propagateBasicBound(f.a.boundByBoolean[bv], true))
	lit, err := f.s.Literal(target)
	astTestMust(t, err)
	value, err := f.s.Assignment(lit)
	astTestMust(t, err)
	if value != lbool.Undef || f.a.basicPropagations != 0 || f.a.lpPropagations != 0 {
		t.Fatal("zero threshold did not disable both propagation paths")
	}
}

func TestSMTLRALPPropagationAllocationCleanup(t *testing.T) {
	refusals, successes := 0, 0
	for _, stop := range []int64{-1, 0, 1, 2, 5, 12, 32} {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newLRAFrontendFixture(t, true)
			astTestMust(t, f.a.numeric.Close())
			astTestMust(t, f.a.numerals.CloseChecked())
			budget := arena.NewBudget(8 << 20)
			m, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
				Cells: arena.Config{PageSize: 4, Budget: budget}, Words: arena.VectorConfig{PageSize: 32, Descriptors: arena.Config{PageSize: 4, Budget: budget}},
			}, Synchronized: true})
			astTestMust(t, err)
			f.a.numerals = m
			f.a.numeric, err = newSMTLRANumeric(m, smtLRANumericConfig{strategy: lp.StrategyTableauRows, boundPropagation: true, pivotedRows: true})
			astTestMust(t, err)
			var padding []numeral.Rational
			t.Cleanup(func() {
				budget.FailAfter(-1)
				if f.a.propagator != nil {
					astTestMust(t, f.a.propagator.Close())
					f.a.propagator = nil
				}
				astTestMust(t, f.a.numeric.Close())
				for i := range padding {
					astTestMust(t, m.Del(&padding[i]))
				}
				lraNumericManagerCensus(t, m)
				astTestMust(t, f.a.Close())
				if budget.Used() != 0 {
					t.Fatal("LRA propagation retained private budget", budget.Used())
				}
			})
			large := new(big.Rat).SetFrac(new(big.Int).Exp(big.NewInt(10), big.NewInt(300), nil), big.NewInt(7))
			target := prepareLRALPPropagationScale(t, f, large)
			// Keep available numeric cells live so a large row explanation
			// crosses an actual slab frontier instead of reusing warm cells.
			for i := 0; i < 64; i++ {
				stats := m.Z.Cells.Stats()
				if stats.Slots-stats.Live-stats.Retired <= 1 {
					break
				}
				v := numeral.NewRational()
				astTestMust(t, m.SetString(&v, large.RatString()))
				padding = append(padding, v)
			}
			stats := m.Z.Cells.Stats()
			if stats.Slots-stats.Live-stats.Retired > 1 {
				t.Fatal("failed to fill numeric cell frontier", stats)
			}
			budget.FailAfter(stop)
			err = f.a.propagateLPBounds()
			budget.FailAfter(-1)
			if err != nil {
				// Checked numeral allocation can route a physical refusal
				// through the source Runtime's typed memout transport.
				var failure umemory.Failure
				memout := errors.As(err, &failure) && failure.Kind == umemory.FailureMemout && failure.ErrorCode() == 101 && stop >= 0 && budget.Used() < budget.Limit()
				if !errors.Is(err, arena.ErrInjectedFailure) && !memout {
					t.Fatalf("unexpected allocation error %T: %#v", err, err)
				}
				refusals++
				if f.a.propagator == nil {
					t.Fatal("refused propagator lost cleanup owner")
				}
				if next := f.a.Propagate(); next != lp.ErrExactVectorPending {
					t.Fatal("stopped propagation reused", next)
				}
			} else {
				successes++
				lit, err := f.s.Literal(target)
				astTestMust(t, err)
				value, err := f.s.Assignment(lit)
				astTestMust(t, err)
				if value != lbool.True {
					t.Fatal("completed propagation lost result", value)
				}
			}
		})
	}
	if refusals == 0 || successes == 0 {
		t.Fatal("allocation samples need both failure and success", refusals, successes)
	}
	t.Log("physical allocation samples", "refusals", refusals, "successes", successes)
}

// AUDIT.md smtcore_11-F4: updt_unassigned_bounds pushes a
// vector_value_trail entry per change and push_scope_eh records only sizes,
// so a scope no longer copies the whole m_unassigned_bounds vector.
func TestSMTLRAUnassignedBoundsTrailRestore(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("lra-unassigned-x"), f.variable("lra-unassigned-y")
	astTestMust(t, f.s.Internalize(f.app(smtArithOpLE, x, f.number(1)), true))
	astTestMust(t, f.s.Internalize(f.app(smtArithOpGE, y, f.number(2)), true))
	vx := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())
	vy := f.s.context.enodes.get(y).TheoryVar(f.a.FamilyID())
	if vx == SMTNullTheoryVar || vy == SMTNullTheoryVar {
		t.Fatal("missing theory variables")
	}
	base := append([]uint32(nil), f.a.unassigned...)
	baseTrail := len(f.a.unassignedTrail)
	astTestMust(t, f.s.PushScope())
	astTestMust(t, f.a.updateUnassignedBounds(vx, -1))
	astTestMust(t, f.s.PushScope())
	astTestMust(t, f.a.updateUnassignedBounds(vx, 3))
	astTestMust(t, f.a.updateUnassignedBounds(vy, -1))
	z := f.variable("lra-unassigned-z")
	astTestMust(t, f.s.Internalize(f.app(smtArithOpLE, z, f.number(4)), true))
	vz := f.s.context.enodes.get(z).TheoryVar(f.a.FamilyID())
	if f.a.unassigned[vx] != base[vx]+2 || f.a.unassigned[vy] != base[vy]-1 || f.a.unassigned[vz] == 0 {
		t.Fatal("unassigned updates", f.a.unassigned)
	}
	for _, scope := range f.a.scopes {
		if scope.unassigned > len(f.a.unassigned) {
			t.Fatal("scope records a size, not a copy")
		}
	}
	astTestMust(t, f.s.PopScopes(1))
	if f.a.unassigned[vx] != base[vx]-1 || f.a.unassigned[vy] != base[vy] {
		t.Fatal("first pop", f.a.unassigned)
	}
	if int(vz) < len(f.a.unassigned) && f.a.unassigned[vz] != 0 {
		t.Fatal("variable created in the popped scope kept its count", f.a.unassigned)
	}
	astTestMust(t, f.s.PopScopes(1))
	for i, value := range base {
		if f.a.unassigned[i] != value {
			t.Fatal("second pop", f.a.unassigned, base)
		}
	}
	if len(f.a.unassignedTrail) != baseTrail {
		t.Fatal("trail not unwound", len(f.a.unassignedTrail))
	}
}
