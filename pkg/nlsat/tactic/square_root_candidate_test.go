package tactic

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	basetactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Enumerate small finite domains independently of the signed-square
// comparison and propagation algorithms. The small radicands give well
// separated floating values, and identical radicands use the same square root.
func TestSquareRootOrderAgainstEverySignAssignment(t *testing.T) {
	f := newGoalCompilerFixture(t)
	rng := rand.New(rand.NewSource(114591))
	for trial := 0; trial < 1400; trial++ {
		n := 2 + rng.Intn(6)
		initial := make([]signedSquareRoot, n)
		for i := range initial {
			c := int32(rng.Intn(8))
			initial[i] = signedSquareRoot{c, c != 0}
		}
		term := func() candidateTerm {
			v := int32(rng.Intn(n))
			switch rng.Intn(7) {
			case 0:
				return candidateTerm{kind: candidateSquare, index: v}
			case 1, 2:
				return candidateTerm{kind: candidateInteger, index: int32(rng.Intn(9) - 4)}
			default:
				return candidateTerm{kind: candidateConstant, index: v}
			}
		}
		var rows []candidateComparison
		for j := rng.Intn(14) + 1; j > 0; j-- {
			op := [...]int32{candidateEqual, candidateLT, candidateGT, candidateLE, candidateGE}[rng.Intn(5)]
			rows = append(rows, candidateComparison{left: term(), right: term(), operation: op})
		}
		satisfies := func(mask int) bool {
			value := func(t candidateTerm) float64 {
				if t.kind == candidateInteger {
					return float64(t.index)
				}
				square := float64(initial[t.index].square)
				if t.kind == candidateSquare {
					return square
				}
				x := math.Sqrt(square)
				if mask&(1<<t.index) == 0 {
					return -x
				}
				return x
			}
			for _, c := range rows {
				l, r := value(c.left), value(c.right)
				switch c.operation {
				case candidateEqual:
					if l != r {
						return false
					}
				case candidateLT:
					if l >= r {
						return false
					}
				case candidateLE:
					if l > r {
						return false
					}
				case candidateGT:
					if l <= r {
						return false
					}
				case candidateGE:
					if l < r {
						return false
					}
				}
			}
			return true
		}
		hasSolution := false
		for mask := 0; mask < 1<<n; mask++ {
			if satisfies(mask) {
				hasSolution = true
				break
			}
		}
		p := integerCandidate{manager: f.manager, comparisons: rows}
		out := append([]signedSquareRoot(nil), initial...)
		solved := p.assignSquareRoots(out) == lbool.True
		if solved != hasSolution {
			t.Fatalf("trial %d, domains %v rows %v, solved %v want %v", trial, initial, rows, solved, hasSolution)
		}
		mask := 0
		for i, v := range out {
			if !v.negative {
				mask |= 1 << i
			}
		}
		if solved && !satisfies(mask) {
			t.Fatalf("trial %d produced invalid signs %b", trial, mask)
		}
	}
}

func TestSquareRootOrderCancellation(t *testing.T) {
	f := newGoalCompilerFixture(t)
	p := integerCandidate{manager: f.manager, comparisons: []candidateComparison{{
		left: candidateTerm{kind: candidateConstant}, right: candidateTerm{kind: candidateInteger},
		operation: candidateEqual,
	}}}
	roots := []signedSquareRoot{{square: 2, negative: true}}
	f.manager.Limit.Cancel()
	defer f.manager.Limit.DecCancel()
	if got := p.assignSquareRoots(roots); got != lbool.Undef || !roots[0].negative {
		t.Fatalf("canceled propagation returned %v with roots %v", got, roots)
	}
}

func TestSquareRootPreparationPublishesExactValues(t *testing.T) {
	if ast.ASTSourceDebug || diagnostic.TraceCompiled() {
		t.Skip("preparation uses the ordinary release configuration")
	}
	for _, conflict := range []bool{false, true} {
		t.Run(fmt.Sprintf("conflict=%v", conflict), func(t *testing.T) {
			f := newGoalCompilerFixture(t)
			m := f.manager
			g, err := basetactic.NewGoal(m, false, true, false)
			compilerMust(t, err)
			t.Cleanup(func() { compilerMust(t, g.Close()) })
			zero, err := m.ArithmeticNumeralInt64(0, false)
			compilerMust(t, err)
			sort, err := m.GetSort(zero)
			compilerMust(t, err)
			var xs []ast.ASTHandle
			for i := 0; i < minCandidateAssertions; i++ {
				x := compilerConst(t, m, fmt.Sprintf("root%d", i), sort)
				xs = append(xs, x)
				sq, err := m.MkArithmeticMul(x, x)
				compilerMust(t, err)
				value, err := m.ArithmeticNumeralInt64(int64(2+i), false)
				compilerMust(t, err)
				eq, err := m.MkEq(value, sq)
				compilerMust(t, err)
				compilerMust(t, g.Assert(eq))
			}
			// Reverse row order makes a forced sign propagate through the entire chain.
			for i := len(xs) - 1; i > 0; i-- {
				link, err := m.MkArithmeticLT(xs[i-1], xs[i])
				compilerMust(t, err)
				compilerMust(t, g.Assert(link))
			}
			pos, err := m.MkArithmeticGT(xs[0], zero)
			compilerMust(t, err)
			compilerMust(t, g.Assert(pos))
			if conflict {
				neg, err := m.MkArithmeticLT(xs[len(xs)-1], zero)
				compilerMust(t, err)
				compilerMust(t, g.Assert(neg))
			}
			original, err := basetactic.NewGoalCopy(g)
			compilerMust(t, err)
			t.Cleanup(func() { compilerMust(t, original.Close()) })
			tac, err := NewPolynomialPreparationTactic(m, f.context, f.globals, nil, f.services)
			compilerMust(t, err)
			t.Cleanup(func() { compilerMust(t, tac.Close()) })
			var result basetactic.TacticResult
			t.Cleanup(func() { compilerMust(t, result.Close()) })
			compilerMust(t, tac.Apply(g, &result))
			if conflict {
				if !result.DecidedUnsat() {
					t.Fatal("expected contradiction in root order")
				}
				return
			}
			if !result.DecidedSat() || result.ModelConverter == nil {
				t.Fatal("missing exact root model")
			}
			var returned basetactic.ModelRef
			compilerMust(t, result.ModelConverter.ApplyUnindexed(&returned))
			t.Cleanup(func() { compilerMust(t, returned.Close()) })
			core, err := returned.Get()
			compilerMust(t, err)
			ev, err := model.NewModelEvaluator(core, f.services)
			compilerMust(t, err)
			t.Cleanup(func() { compilerMust(t, ev.Close()) })
			compilerMust(t, ev.SetModelCompletion(false))
			out, err := m.NewRef(ast.ASTHandle{})
			compilerMust(t, err)
			t.Cleanup(func() { compilerMust(t, m.DelRef(&out)) })
			forms, err := original.AppendFormulas(nil)
			compilerMust(t, err)
			for i, h := range forms {
				compilerMust(t, ev.Evaluate(h, out))
				value, err := m.RefGet(out)
				compilerMust(t, err)
				if !m.IsTrue(value) {
					t.Fatalf("original assertion %d evaluated to %v", i, value)
				}
			}
		})
	}
}
