package smtcore

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTRDLSetupSelectsConcreteBackend(t *testing.T) {
	for _, tc := range []struct {
		name                                                 string
		dense, boolean, ite, push, proofs, simplex, disabled bool
		backend                                              string
	}{
		{name: "sparse", backend: "simplex"},
		{name: "graph", boolean: true, backend: "graph"},
		{name: "ite", boolean: true, ite: true, backend: "simplex"},
		{name: "dense", dense: true, backend: "dense"},
		{name: "dense-simplex", dense: true, simplex: true, backend: "simplex"},
		{name: "dense-proofs", dense: true, proofs: true, backend: "simplex"},
		{name: "dense-first-push", dense: true, push: true, backend: "simplex"},
		{name: "graph-first-push", boolean: true, push: true, backend: "simplex"},
		{name: "graph-proofs", boolean: true, proofs: true, backend: "simplex"},
		{name: "graph-simplex", boolean: true, simplex: true, backend: "simplex"},
		{name: "disabled", boolean: true, disabled: true, backend: "simplex"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			if tc.proofs {
				astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			}
			solver, err := NewSMTSolver(m, nil, "QF_RDL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			if tc.disabled {
				smtDisableAutoConfig(t, solver)
			}
			k := solver.kernel
			k.smtParams.arith.state.AutoConfigSimplex = tc.simplex
			k.arithState.AutoConfigSimplex = tc.simplex
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			real, err := m.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "rdl-profile-x"), real)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "rdl-profile-y"), real)
			astTestMust(t, err)
			fixture := smtRealDiffFixture{manager: m, family: family, sort: real, x: x, y: y}
			if tc.boolean {
				p := smtRelevancyBool(t, m, "rdl-profile-p")
				astTestMust(t, solver.Assert(p))
				if tc.ite {
					term, err := m.MkIte(p, x, y)
					astTestMust(t, err)
					bound, err := m.MkFamilyApp(family, smtArithOpLE, nil, []ast.ASTHandle{term, fixture.numeral(t, 1, 1)})
					astTestMust(t, err)
					astTestMust(t, solver.Assert(bound))
				}
			}
			count := int64(1)
			if tc.dense {
				count = 24
			}
			for i := int64(0); i < count; i++ {
				astTestMust(t, solver.Assert(fixture.bound(t, smtArithOpLE, x, y, i+1, 3)))
			}
			if tc.push {
				astTestMust(t, solver.Push())
			}
			status, err := smtSetupProfileCheck(solver)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("RDL profile changed satisfiability", status)
			}
			switch theory := k.internalizer.theories[family].(type) {
			case *SMTDenseDiffTheory:
				if tc.backend != "dense" {
					t.Fatal("unexpected dense backend")
				}
			case *SMTRealDiffTheory:
				if tc.backend != "graph" || !theory.addBinaryBounds {
					t.Fatal("unexpected or unconfigured real graph")
				}
			case *SMTTheoryArith:
				if tc.backend != "simplex" || !(!theory.integerExt) {
					t.Fatal("unexpected arithmetic integrality route")
				}
			default:
				t.Fatalf("unexpected arithmetic backend %T", theory)
			}
			if !tc.disabled {
				if k.relevancy.enabled || !k.arithState.Eq2Ineq || k.arithState.Reflect || k.arithState.PropagateEqs || k.preprocessState.NNFCNF {
					t.Fatal("RDL scalar policy omitted")
				}
				if tc.backend == "graph" && (k.arithState.BoundProp != 0 || k.arithState.PropagationStrategy != 0 || !k.arithState.AddBinaryBounds) {
					t.Fatal("graph policy omitted")
				}
				if tc.dense && !tc.push && (k.smtState.PhaseSelection != 2 || k.smtState.RestartStrategy != 0 || k.smtState.RestartAdaptive) {
					t.Fatal("dense search policy omitted")
				}
			} else if !k.relevancy.enabled || !k.preprocessState.NNFCNF {
				t.Fatal("disabled setup applied RDL tuning")
			}
			astTestMust(t, solver.Push())
			astTestMust(t, solver.Assert(fixture.bound(t, smtArithOpLE, y, x, -2, 3)))
			status, err = smtSetupProfileCheck(solver)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("RDL child contradiction", status)
			}
			astTestMust(t, solver.Pop(1))
			status, err = smtSetupProfileCheck(solver)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("RDL pop recovery", status)
			}
			if tc.push {
				astTestMust(t, solver.Pop(1))
			}
		})
	}
}

func TestSMTRDLStaticSetupRejectsWrongFeatures(t *testing.T) {
	for _, kind := range []string{"non-difference", "integer", "uf"} {
		t.Run(kind, func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			solver, err := NewSMTSolver(m, nil, "QF_RDL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			one, err := m.ArithmeticRealNumeralText("1")
			astTestMust(t, err)
			if kind == "integer" {
				one, err = m.ArithmeticIntegerNumeralText("1")
				astTestMust(t, err)
			}
			sort, err := m.GetSort(one)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "rdl-invalid-x"), sort)
			astTestMust(t, err)
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			term, want := x, "has integer variables"
			if kind == "non-difference" {
				y, err := m.MkConst(astTestSymbol(t, m, "rdl-invalid-y"), sort)
				astTestMust(t, err)
				term, err = m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{x, y})
				astTestMust(t, err)
				want = "not in QF_RDL"
			} else if kind == "uf" {
				f, err := m.MkFuncDecl(astTestSymbol(t, m, "rdl-invalid-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
				astTestMust(t, err)
				term, err = m.MkApp(f, []ast.ASTHandle{x})
				astTestMust(t, err)
				want = "uninterpreted function symbols"
			}
			bound, err := m.MkFamilyApp(family, smtArithOpLE, nil, []ast.ASTHandle{term, one})
			astTestMust(t, err)
			astTestMust(t, solver.Assert(bound))
			_, err = smtSetupProfileCheck(solver)
			if err == nil || !strings.Contains(err.Error(), want) || solver.kernel.preprocessingSetupDone {
				t.Fatal("invalid RDL features accepted", err, want)
			}
		})
	}
}
