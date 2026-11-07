package smtcore

// Source: src/smt/smt_setup.cpp setup_unknown(static_features&).

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTUnknownStaticSetupClassifiesLinearArithmetic(t *testing.T) {
	for _, test := range []struct {
		name    string
		integer bool
		uf      bool
	}{
		{name: "real"},
		{name: "integer", integer: true},
		{name: "uf-real", uf: true},
		{name: "uf-integer", integer: true, uf: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			sortKind := int32(0)
			if test.integer {
				sortKind = 1
			}
			sort, err := m.MkFamilySort(family, sortKind, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "unknown-static-x-"+test.name), sort)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "unknown-static-y-"+test.name), sort)
			astTestMust(t, err)
			term := x
			if test.uf {
				function, err := m.MkFuncDecl(astTestSymbol(t, m, "unknown-static-f-"+test.name), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
				astTestMust(t, err)
				term, err = m.MkApp(function, []ast.ASTHandle{x})
				astTestMust(t, err)
			}
			sum, err := m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{term, y})
			astTestMust(t, err)
			zero, err := m.ArithmeticNumeralRat(big.NewRat(0, 1), test.integer)
			astTestMust(t, err)
			bound, err := m.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{sum, zero})
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			astTestMust(t, solver.Assert(bound))
			status, err := smtSetupProfileCheck(solver)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("linear arithmetic result", status)
			}
			installed := solver.kernel.internalizer.theories[family]
			if test.integer {
				theory, ok := smtArithFactsOf(installed)
				if !ok || !theory.integer || theory.mixed {
					t.Fatalf("integer classifier installed %T (integer=%v mixed=%v)", installed, ok && theory.integer, ok && theory.mixed)
				}
				if test.uf && solver.kernel.arithState.PropagationThreshold != 1000 {
					t.Fatal("QF_UFLIA propagation threshold", solver.kernel.arithState.PropagationThreshold)
				}
				return
			}
			if _, ok := installed.(*SMTLRATheory); !ok {
				t.Fatalf("real classifier installed %T", installed)
			}
		})
	}
}
