package smtcore

import (
	"fmt"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// A small independent DPLL oracle checks existential auxiliary assignments.
// It consumes only the emitted clauses, never PB propagation or AST evaluation.
func smtPBNetworkSAT(clauses [][]SMTLiteral, values []int8) bool {
	for {
		changed := false
		choice := int32(-1)
		for _, clause := range clauses {
			unit := SMTNullLiteral
			open := 0
			satisfied := false
			for _, l := range clause {
				v := values[l.Var()]
				if v == 0 {
					open++
					unit = l
					continue
				}
				if (v > 0) != l.Sign() {
					satisfied = true
					break
				}
			}
			if satisfied {
				continue
			}
			if open == 0 {
				return false
			}
			if open == 1 {
				v := int8(1)
				if unit.Sign() {
					v = -1
				}
				values[unit.Var()] = v
				changed = true
			} else if choice < 0 {
				choice = unit.Var()
			}
		}
		if changed {
			continue
		}
		if choice < 0 {
			return true
		}
		for _, v := range []int8{1, -1} {
			copyValues := append([]int8{}, values...)
			copyValues[choice] = v
			if smtPBNetworkSAT(clauses, copyValues) {
				return true
			}
		}
		return false
	}
}

func TestSMTPBCompilationPigeonholeSearchAndScopeRecovery(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterPseudoBooleanPlugin()
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()); astTestMust(t, params.Close()) })
	astTestMust(t, solver.Push())
	var xs [7][6]ast.ASTHandle
	for i := range xs {
		for j := range xs[i] {
			xs[i][j] = smtRelevancyBool(t, m, fmt.Sprintf("pigeon-%d-%d", i, j))
		}
		row, err := m.MkOrArgs(xs[i][:])
		astTestMust(t, err)
		astTestMust(t, solver.Assert(row))
	}
	for j := 0; j < 6; j++ {
		var column []ast.ASTHandle
		for i := range xs {
			column = append(column, xs[i][j])
		}
		h, err := m.MkPseudoBooleanAtMost(column, 1)
		astTestMust(t, err)
		astTestMust(t, solver.Assert(h))
	}
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("seven pigeons fit into six holes", status)
	}
	pb := solver.kernel.internalizer.theories[family].(*SMTPseudoBooleanTheory)
	if pb.statistics.compiled == 0 || pb.statistics.compiledClauses == 0 || solver.kernel.internalizer.statistics.NumRestarts == 0 {
		t.Fatal("search did not exercise deferred PB compilation", pb.statistics, solver.kernel.internalizer.statistics.NumRestarts)
	}
	astTestMust(t, solver.Pop(1))
	status, err = solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("compiled pigeonhole circuit leaked past pop", status)
	}
}

func smtPBNetworkClauses(s *SMTInternalizerState) [][]SMTLiteral {
	var out [][]SMTLiteral
	for _, c := range s.clauses {
		if c.clause == nil || !c.clause.Deleted() {
			out = append(out, append([]SMTLiteral{}, c.Literals...))
		}
	}
	for _, l := range s.assignedLiterals {
		out = append(out, []SMTLiteral{l})
	}
	return out
}

func TestSMTPBCompilationNetworkIndependentTruthOracle(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6, 7, 10, 11, 17} {
		for k := 0; k <= n+1; k++ {
			for _, full := range []bool{false, true} {
				t.Run(fmt.Sprintf("n%d/k%d/full%v", n, k, full), func(t *testing.T) {
					m := astTestManager(t)
					family, err := m.RegisterPseudoBooleanPlugin()
					astTestMust(t, err)
					_, _, s := smtInternalizerFixture(t, m, nil)
					pb, err := NewSMTPseudoBooleanTheory(family)
					astTestMust(t, err)
					astTestMust(t, s.RegisterTheory(pb))
					var inputs []SMTLiteral
					for i := 0; i < n; i++ {
						h := smtRelevancyBool(t, m, fmt.Sprintf("sort-%d", i))
						astTestMust(t, s.Internalize(h, true))
						l, err := s.Literal(h)
						astTestMust(t, err)
						if i%3 == 0 {
							l = l.Negated()
						}
						inputs = append(inputs, l)
					}
					network := smtPBSort{theory: pb}
					out := network.ge(full, uint32(k), inputs)
					astTestMust(t, network.err)
					if n == 5 && k == 1 && !full && (network.variables != 1 || network.clauses != 1) {
						t.Fatal("source cardinality cost did not select direct sorting", network.variables, network.clauses)
					}
					clauses := smtPBNetworkClauses(s)
					patterns := 1 << n
					if n > 7 {
						patterns = 32
					}
					for pattern := 0; pattern < patterns; pattern++ {
						mask := pattern
						if n > 7 {
							mask = (pattern * 7919) & ((1 << n) - 1)
							if pattern == 1 {
								mask = (1 << n) - 1
							}
						}
						count := 0
						values := make([]int8, len(s.boolExpr))
						values[0] = 1
						for i, l := range inputs {
							value := mask&(1<<i) != 0
							if value {
								count++
							}
							values[l.Var()] = -1
							if value != l.Sign() {
								values[l.Var()] = 1
							}
						}
						for _, positive := range []bool{false, true} {
							forced := out
							if !positive {
								forced = forced.Negated()
							}
							cs := append(append([][]SMTLiteral{}, clauses...), []SMTLiteral{forced})
							got := smtPBNetworkSAT(cs, append([]int8{}, values...))
							want := !positive || count >= k
							if full || k == 0 || k > n {
								want = positive == (count >= k)
							}
							if got != want {
								t.Fatalf("mask=%b count=%d output=%v: SAT=%v want=%v network vars=%d clauses=%d", mask, count, positive, got, want, network.variables, network.clauses)
							}
						}
					}
				})
			}
		}
	}
}

func TestSMTPBCompilationWeightedDefinitionsAndBaseFolding(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, partial := range []bool{false, true} {
			t.Run(fmt.Sprintf("proof%v/partial%v", proofs, partial), func(t *testing.T) {
				m, s, pb, atom, inputs := smtPBWatchFixture(t, []int64{4, 3, 2, 2, 1}, 6, false)
				if proofs {
					astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
				}
				astTestMust(t, s.PushScope())
				if partial {
					smtPBResolutionRoot(t, s, atom, proofs)
				}
				astTestMust(t, pb.compileInequality(pb.constraints[0]))
				clauses := smtPBNetworkClauses(s)
				for mask := 0; mask < 32; mask++ {
					sum := 0
					for i, w := range []int{4, 3, 2, 2, 1} {
						if mask&(1<<i) != 0 {
							sum += w
						}
					}
					for _, positive := range []bool{false, true} {
						values := make([]int8, len(s.boolExpr))
						values[0] = 1
						for i, l := range inputs {
							values[l.Var()] = -1
							if mask&(1<<i) != 0 {
								values[l.Var()] = 1
							}
						}
						values[atom.Var()] = -1
						if positive {
							values[atom.Var()] = 1
						}
						got := smtPBNetworkSAT(clauses, values)
						want := positive == (sum >= 6)
						if partial {
							want = positive && sum >= 6
						}
						if got != want {
							t.Fatal("weighted definition", mask, positive, got, want)
						}
					}
				}
				if proofs {
					// Auxiliary clause justifications are scope-owned
					// (m_justifications); the clause still carries one.
					for _, clause := range s.clauses {
						if clause.clause == nil || clause.clause.Justification() == nil {
							t.Fatal("compiled proof clause has no theory axiom")
						}
					}
				}
				astTestMust(t, s.PopScopes(1))
			})
		}
	}
	// Source's strict k<m early return leaves the request undefined and
	// counts an attempted compilation, without installing a compiled trail.
	_, s, pb, _, inputs := smtPBWatchFixture(t, []int64{4, 3, 2, 2, 1}, 6, false)
	astTestMust(t, s.AssignLiteral(inputs[0], NewSMTBooleanAxiom()))
	astTestMust(t, s.AssignLiteral(inputs[1], NewSMTBooleanAxiom()))
	c := pb.constraints[0]
	c.propagations = c.compilationThreshold - 1
	pb.incPropagations(c)
	astTestMust(t, pb.Restart())
	if c.compiled != lbool.Undef || pb.statistics.compiled != 1 || pb.statistics.compiledVariables != 0 || len(s.clauses) != 0 || len(pb.toCompile) != 0 {
		t.Fatal("source early-return lifecycle", c.compiled, pb.statistics, len(s.clauses))
	}
	// Equality at the boundary k==m reaches ge(0), rather than returning.
	_, s2, p2, _, ls := smtPBWatchFixture(t, []int64{4, 3, 2, 2, 1}, 6, false)
	astTestMust(t, s2.AssignLiteral(ls[0], NewSMTBooleanAxiom()))
	astTestMust(t, s2.AssignLiteral(ls[2], NewSMTBooleanAxiom()))
	astTestMust(t, p2.compileInequality(p2.constraints[0]))
	if p2.constraints[0].compiled != lbool.True || p2.statistics.compiledVariables != 0 {
		t.Fatal("k==m did not compile constant-true residual")
	}
}

func TestSMTPBCompilationThresholdRestartAndRollback(t *testing.T) {
	// A real weighted propagation, rather than the test's counter helper,
	// must queue the source restart callback at precisely the threshold.
	_, native, nativePB, nativeAtom, nativeInputs := smtPBWatchFixture(t, []int64{4, 3, 2, 2, 1}, 6, false)
	nativeConstraint := nativePB.constraints[0]
	nativeConstraint.propagations = nativeConstraint.compilationThreshold - 1
	smtPBWatchAssign(t, native, nativeAtom)
	smtPBWatchAssign(t, native, nativeInputs[0].Negated())
	if len(nativePB.toCompile) != 1 || nativePB.statistics.compiled != 0 {
		t.Fatal("native propagation failed to schedule deferred compilation")
	}
	astTestMust(t, native.RestartTheories())
	if nativePB.statistics.compiled != 1 || nativeConstraint.compiled != lbool.True {
		t.Fatal("theory restart dispatcher skipped PB compilation")
	}

	_, s, pb, _, _ := smtPBWatchFixture(t, []int64{4, 3, 2, 2, 1}, 6, false)
	c := pb.constraints[0]
	if c.compilationThreshold != 80 || c.compiled != lbool.False {
		t.Fatal("source threshold", c.compilationThreshold, c.compiled)
	}
	for i := uint32(0); i < c.compilationThreshold-1; i++ {
		pb.incPropagations(c)
	}
	if len(pb.toCompile) != 0 {
		t.Fatal("compiled before source threshold")
	}
	pb.incPropagations(c)
	pb.incPropagations(c)
	if len(pb.toCompile) != 1 || c.compiled != lbool.Undef || pb.statistics.compiled != 0 {
		t.Fatal("pending compilation was duplicated or eager")
	}
	astTestMust(t, s.PushScope())
	astTestMust(t, pb.Restart())
	if c.compiled != lbool.True || len(pb.toCompile) != 0 || pb.statistics.compiled != 1 || pb.statistics.compiledVariables == 0 || pb.statistics.compiledClauses == 0 {
		t.Fatal("restart failed to compile", pb.statistics)
	}
	for _, clause := range s.clauses {
		if clause.clause != nil && clause.clause.Kind() != SMTClauseAux {
			t.Fatal("network became learned lemma")
		}
	}
	astTestMust(t, s.PopScopes(1))
	if c.compiled != lbool.False || len(s.clauses) != 0 {
		t.Fatal("compiled circuit survived scope pop", c.compiled, len(s.clauses))
	}
	pb.incPropagations(c)
	if len(pb.toCompile) != 1 {
		t.Fatal("surviving inequality was not requeued")
	}
	astTestMust(t, pb.Restart())
	if pb.statistics.compiled != 2 {
		t.Fatal("surviving inequality was not recompiled")
	}

	for _, test := range []struct {
		weights     []int64
		bound       int64
		eq, disable bool
	}{
		{[]int64{9, 8, 7, 2}, 15, false, false},
		{[]int64{4, 3, 2}, 6, true, false},
		{[]int64{4, 3, 2}, 6, false, true},
	} {
		_, _, p, _, _ := smtPBWatchFixture(t, test.weights, test.bound, test.eq)
		if test.disable {
			p.controls.EnableCompilation = false
			p.initCompilation(p.constraints[0])
		}
		if p.constraints[0].compilationThreshold != math.MaxUint32 {
			t.Fatal("ineligible inequality received threshold")
		}
	}

	_, s2, p2, _, _ := smtPBWatchFixture(t, []int64{4, 3, 2}, 6, false)
	astTestMust(t, s2.PushScope())
	h := smtPBResolveExpression(t, s2.manager, p2.FamilyID(), []ast.ASTHandle{smtRelevancyBool(t, s2.manager, "local-a"), smtRelevancyBool(t, s2.manager, "local-b"), smtRelevancyBool(t, s2.manager, "local-c")}, []int32{2, 2, 1}, 3)
	astTestMust(t, s2.Internalize(h, true))
	local := p2.constraints[1]
	local.propagations = local.compilationThreshold - 1
	p2.incPropagations(local)
	if len(p2.toCompile) != 1 {
		t.Fatal("local constraint not queued")
	}
	astTestMust(t, s2.PopScopes(1))
	if len(p2.toCompile) != 0 {
		t.Fatal("pending pointer to deleted inequality survived pop")
	}
}

func TestSMTPBPropagationCounterWrapPrecedesCompilationThreshold(t *testing.T) {
	_, _, pb, _, _ := smtPBWatchFixture(t, []int64{4, 3, 2}, 6, false)
	c := pb.constraints[0]
	c.propagations = math.MaxUint32
	pb.incPropagations(c)
	if c.propagations != 0 || len(pb.toCompile) != 0 || c.compiled != lbool.False {
		t.Fatal("source unsigned propagation wrap changed compilation scheduling", c.propagations, c.compiled)
	}
	pb.incPropagations(c)
	if c.propagations != 1 || len(pb.toCompile) != 0 {
		t.Fatal("post-wrap threshold state")
	}
}
