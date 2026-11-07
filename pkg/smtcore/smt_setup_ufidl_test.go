package smtcore

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTUFIDLSetupProfilesAndBackends(t *testing.T) {
	for _, tc := range []struct {
		name                                    string
		functions, dense, push, proofs, simplex bool
	}{
		{name: "sparse"},
		{name: "uf", functions: true},
		{name: "uf-proof", functions: true, proofs: true},
		{name: "dense", dense: true},
		{name: "dense-simplex-flag", dense: true, simplex: true},
		{name: "dense-proof", dense: true, proofs: true},
		{name: "dense-uf", dense: true, functions: true},
		{name: "dense-first-push", dense: true, push: true},
		{name: "uf-first-push", functions: true, push: true},
		{name: "proof-first-push", proofs: true, push: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			if tc.proofs {
				astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			}
			solver, err := NewSMTSolver(m, nil, "QF_UFIDL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			k := solver.kernel
			k.smtParams.arith.state.AutoConfigSimplex = tc.simplex
			k.arithState.AutoConfigSimplex = tc.simplex
			originalSMT, originalArith := k.smtState, k.arithState
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			integer, err := m.MkFamilySort(family, 1, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "ufidl-profile-x"), integer)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "ufidl-profile-y"), integer)
			astTestMust(t, err)
			fixture := smtIntegerDiffFixture{manager: m, family: family, sort: integer, x: x, y: y}
			count := int64(1)
			if tc.dense {
				count = 48
			}
			for i := int64(0); i < count; i++ {
				astTestMust(t, solver.Assert(fixture.bound(t, smtArithOpLE, x, y, i+1)))
			}
			if tc.functions {
				f, err := m.MkFuncDecl(astTestSymbol(t, m, "ufidl-profile-f"), []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{})
				astTestMust(t, err)
				fx, err := m.MkApp(f, []ast.ASTHandle{x})
				astTestMust(t, err)
				fy, err := m.MkApp(f, []ast.ASTHandle{y})
				astTestMust(t, err)
				equality, err := m.MkEq(fx, fy)
				astTestMust(t, err)
				different, err := m.MkNot(equality)
				astTestMust(t, err)
				astTestMust(t, solver.Assert(different))
			}
			if tc.push {
				astTestMust(t, solver.Push())
			}
			status, err := smtSetupProfileCheck(solver)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("UFIDL profile changed satisfiability", status)
			}
			dense := tc.dense && !tc.functions && !tc.push
			_, gotDense := k.internalizer.theories[family].(*SMTDenseDiffTheory)
			if gotDense != (dense && !tc.proofs) {
				t.Fatal("UFIDL dense backend routing", gotDense)
			}
			if !gotDense {
				theory, ok := smtArithFactsOf(k.internalizer.theories[family])
				if !ok || theory.mixed != (tc.proofs && !tc.push) || theory.integer == (tc.proofs && !tc.push) {
					t.Fatal("UFIDL simplex integrality mode")
				}
			}
			wantEq2Ineq := !tc.functions || tc.push
			wantPropagate := tc.functions || tc.push
			if k.arithState != k.smtParams.arith.state || k.arithState.Eq2Ineq != wantEq2Ineq || k.arithState.PropagateEqs != wantPropagate || k.arithState.Reflect || k.relevancy.enabled || k.preprocessState.NNFCNF {
				t.Fatal("UFIDL owner/consumer policy mismatch", k.arithState)
			}
			if dense {
				if k.arithState.EqBounds != originalArith.EqBounds || k.arithState.SmallLemmaSize != 128 || !k.smtState.LemmaGcHalf || k.smtState.PhaseSelection != originalSMT.PhaseSelection || k.smtState.RestartFactor != originalSMT.RestartFactor || k.smtState.RestartAdaptive != originalSMT.RestartAdaptive {
					t.Fatal("dense UFIDL did not preserve early-return settings")
				}
			} else if !k.arithState.EqBounds || k.smtState.PhaseSelection != 0 || k.smtState.RestartFactor != 1.5 || k.smtState.RestartAdaptive {
				t.Fatal("UFIDL non-dense restart/equality-bound policy")
			}
			astTestMust(t, solver.Push())
			astTestMust(t, solver.Assert(fixture.bound(t, smtArithOpLE, y, x, -2)))
			status, err = smtSetupProfileCheck(solver)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("UFIDL scoped contradiction", status)
			}
			astTestMust(t, solver.Pop(1))
			status, err = smtSetupProfileCheck(solver)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("UFIDL pop recovery", status)
			}
			if tc.push {
				astTestMust(t, solver.Pop(1))
			}
		})
	}
}

func TestSMTUFIDLStaticValidationMatchesSource(t *testing.T) {
	for _, real := range []bool{false, true} {
		name := "linear-counter-example"
		if real {
			name = "reject-real"
		}
		t.Run(name, func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			solver, err := NewSMTSolver(m, nil, "QF_UFIDL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			one, err := m.ArithmeticIntegerNumeralText("1")
			astTestMust(t, err)
			if real {
				one, err = m.ArithmeticRealNumeralText("1")
				astTestMust(t, err)
			}
			sort, err := m.GetSort(one)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "ufidl-validation-x"), sort)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "ufidl-validation-y"), sort)
			astTestMust(t, err)
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			sum, err := m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{x, y})
			astTestMust(t, err)
			bound, err := m.MkFamilyApp(family, smtArithOpLE, nil, []ast.ASTHandle{sum, one})
			astTestMust(t, err)
			astTestMust(t, solver.Assert(bound))
			status, err := smtSetupProfileCheck(solver)
			if real {
				if err == nil || !strings.Contains(err.Error(), "has real variables") || solver.kernel.preprocessingSetupDone {
					t.Fatal("UFIDL real-feature guard", err)
				}
			} else {
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal("source UFIDL setup does not reject linear non-difference input at the kernel API", status)
				}
			}
		})
	}
}
