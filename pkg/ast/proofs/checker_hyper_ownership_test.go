package proofs_test

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestCheckerHyperResolutionOwnsFreshBodyPivots(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			m := fixture(t)
			var roots []ast.ASTRef
			defer func() {
				for i := len(roots) - 1; i >= 0; i-- {
					must(t, m.DelRef(&roots[i]))
				}
			}()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				must(t, err)
				ref, err := m.NewRef(h)
				must(t, err)
				roots = append(roots, ref)
				return h
			}
			left := own(boolean(t, m, "pivot-left"), nil)
			right := own(boolean(t, m, "pivot-right"), nil)
			goal := own(boolean(t, m, "pivot-goal"), nil)
			body := left
			if count == 2 {
				body = own(m.MkAndArgs([]ast.ASTHandle{left, right}))
			}
			clause := own(m.MkImplies(body, goal))
			premises := []ast.ASTHandle{own(m.MkProofAsserted(clause)), own(m.MkProofAsserted(left))}
			positions := []ast.ASTProofPosition{{First: 1, Second: 0}}
			if count == 2 {
				premises = append(premises, own(m.MkProofAsserted(right)))
				positions = append(positions, ast.ASTProofPosition{First: 2, Second: 0})
			}
			proof := own(m.MkProofHyperResolve(premises, goal, positions, make([][]ast.ASTHandle, len(premises))))
			checker, err := proofs.NewChecker(m)
			must(t, err)
			// Warm only declaration kinds. Neither body pivot's negation
			// exists before the first real checker invocation.
			own(m.MkNot(goal))
			own(m.MkIff(left, goal))
			own(m.MkOr(left, goal))
			own(m.MkAndArgs([]ast.ASTHandle{left, goal}))
			counts := func() [4]uint32 {
				s := m.Stats()
				return [4]uint32{s.LiveNodes, s.LiveInfos, s.LiveParameters, s.LiveReferences}
			}
			for attempt := 0; attempt < 2; attempt++ {
				before := counts()
				valid, side, err := checker.Check(proof)
				must(t, err)
				if !valid || len(side) != 1 {
					t.Fatal("valid Horn resolution lost its side condition", valid, len(side))
				}
				must(t, checker.ReleaseSideConditions(side))
				if after := counts(); after != before {
					t.Fatal("checker left a temporary pivot or resolvent", attempt, before, after)
				}
			}
			// Independently discharge the actual returned condition. Its
			// negation must be unsatisfiable, regardless of pivot ownership.
			valid, side, err := checker.Check(proof)
			must(t, err)
			if !valid {
				t.Fatal("repeated Horn proof became invalid")
			}
			defer func() { must(t, checker.ReleaseSideConditions(side)) }()
			params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
			must(t, err)
			defer func() { must(t, params.Close()) }()
			solver, err := smtcore.NewSMTSolver(m, nil, "QF_UF", smtcore.SMTSolverConfig{Params: params})
			must(t, err)
			defer func() { must(t, solver.Close()) }()
			for _, condition := range side {
				must(t, solver.Push())
				must(t, solver.Assert(own(m.MkNot(condition))))
				status, err := solver.CheckSat(nil)
				must(t, err)
				if status != lbool.False {
					t.Fatal("checker changed the resolution condition", status)
				}
				must(t, solver.Pop(1))
			}
		})
	}
}
