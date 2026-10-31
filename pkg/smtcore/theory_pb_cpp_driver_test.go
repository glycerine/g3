package smtcore

// Source: src/test/theory_pb.cpp, constant-literal cardinality matrix.

import (
	"math/bits"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

func TestPseudoBooleanTheorySourceConstantCardinalities(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterPseudoBooleanPlugin()
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "QF_FD", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, solver.Close())
		astTestMust(t, params.Close())
	})
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	for n := 4; n < 11; n++ {
		args := make([]ast.ASTHandle, n)
		for mask := 0; mask < 1<<n; mask++ {
			for i := range args {
				args[i] = falsity
				if mask&(1<<i) != 0 {
					args[i] = truth
				}
			}
			count := uint32(bits.OnesCount(uint(mask)))
			for offset := uint32(0); offset <= 1; offset++ {
				constraint, err := m.MkPseudoBooleanAtLeast(args, count+offset)
				astTestMust(t, err)
				astTestMust(t, solver.Push())
				astTestMust(t, solver.Assert(constraint))
				status, err := solver.CheckSat(nil)
				astTestMust(t, err)
				want := lbool.True
				if offset == 1 {
					want = lbool.False
				}
				if status != want {
					t.Fatalf("n=%d mask=%d bound=%d: got %v want %v", n, mask, count+offset, status, want)
				}
				astTestMust(t, solver.Pop(1))
			}
		}
	}
}

// TestPseudoBooleanTheorySourceNestedFuzz checks a bounded version of the
// source's nested push/check/pop fuzzer against all eight Boolean assignments.
func TestPseudoBooleanTheorySourceNestedFuzz(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterPseudoBooleanPlugin()
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "QF_FD", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()); astTestMust(t, params.Close()) })
	rw := smtQuickTheoryRewriter(t, m)
	firstRewrite, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	secondRewrite, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&secondRewrite)); astTestMust(t, m.DelRef(&firstRewrite)) })
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	var variables, negations [3]ast.ASTHandle
	for i, name := range []string{"source-pb-b0", "source-pb-b1", "source-pb-b2"} {
		variables[i], err = m.MkConst(astTestSymbol(t, m, name), boolean)
		astTestMust(t, err)
		astTestPin(t, m, variables[i])
		negations[i], err = m.MkNot(variables[i])
		astTestMust(t, err)
		astTestPin(t, m, negations[i])
	}
	rng := z3rand.New(0)
	addInequality := func() uint8 {
		var negated [3]bool
		for {
			bound := int32(rng.Bounded(6))
			var weights [3]int32
			for i := range weights {
				weight := int32(3) - int32(rng.Bounded(5))
				if weight < 0 {
					negated[i] = !negated[i]
					weight = -weight
					bound += weight
				}
				weights[i] = weight
			}
			var args [3]ast.ASTHandle
			for i := range args {
				args[i] = variables[i]
				if negated[i] {
					args[i], err = m.MkNot(args[i])
					astTestMust(t, err)
				}
			}
			formula, err := m.MkPseudoBooleanGEIntegers(weights[:], args[:], bound)
			astTestMust(t, err)
			astTestMust(t, rw.Rewrite(formula, firstRewrite))
			first, err := m.RefGet(firstRewrite)
			astTestMust(t, err)
			astTestMust(t, rw.Rewrite(first, secondRewrite))
			second, err := m.RefGet(secondRewrite)
			astTestMust(t, err)
			isGE, err := m.IsPseudoBooleanGE(second)
			astTestMust(t, err)
			if !isGE {
				continue
			}
			// The source asserts the twice-rewritten expression selected above.
			astTestMust(t, solver.Assert(second))
			var allowed uint8
			for assignment := 0; assignment < 8; assignment++ {
				var total int32
				for i, weight := range weights {
					value := assignment&(1<<i) != 0
					if value != negated[i] {
						total += weight
					}
				}
				if total >= bound {
					allowed |= 1 << assignment
				}
			}
			return allowed
		}
	}
	verified := 0
	var runRound func(*uint32, uint32, uint8)
	runRound = func(previous *uint32, level uint32, allowed uint8) {
		astTestMust(t, solver.Push())
		defer func() { astTestMust(t, solver.Pop(1)) }()
		var nested, rounds uint32
		for rounds <= *previous+1 {
			allowed &= addInequality()
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			want := lbool.False
			if allowed != 0 {
				want = lbool.True
			}
			if status != want {
				t.Fatalf("level=%d round=%d mask=%08b: got %v want %v", level, rounds, allowed, status, want)
			}
			if verified < 12 {
				for assignment := 0; assignment < 8; assignment++ {
					astTestMust(t, solver.Push())
					for i := range variables {
						literal := negations[i]
						if assignment&(1<<i) != 0 {
							literal = variables[i]
						}
						astTestMust(t, solver.Assert(literal))
					}
					got, e := solver.CheckSat(nil)
					astTestMust(t, e)
					expected := lbool.False
					if allowed&(1<<assignment) != 0 {
						expected = lbool.True
					}
					if got != expected {
						t.Fatalf("level=%d round=%d assignment=%03b mask=%08b: got %v want %v", level, rounds, assignment, allowed, got, expected)
					}
					astTestMust(t, solver.Pop(1))
				}
				verified++
			}
			rounds++
			if status == lbool.False {
				break
			}
			if level > 0 {
				runRound(&nested, level-1, allowed)
			}
		}
		*previous = rounds
	}
	var rounds uint32
	for i := 0; i < 100; i++ {
		runRound(&rounds, 2, 0xff)
	}
	if verified != 12 {
		t.Fatalf("checked %d nested scopes, want 12", verified)
	}
}
