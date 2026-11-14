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

// smtcore_06-F2: setup::setup_unknown(static_features&) calls the
// featureless setup_QF_AX() for a pure array problem, which leaves the
// relevancy level and phase selection alone even when all assertions are
// units.
func TestSMTUnknownStaticSetupPureArrayKeepsRelevancy(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	index, err := m.MkUninterpretedSort(astTestSymbol(t, m, "unknown-array-U"), nil)
	astTestMust(t, err)
	value, err := m.MkUninterpretedSort(astTestSymbol(t, m, "unknown-array-V"), nil)
	astTestMust(t, err)
	arraySort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{index}), value)
	astTestMust(t, err)
	var arrays []ast.ASTHandle
	for _, name := range []string{"unknown-array-a", "unknown-array-b", "unknown-array-c"} {
		x, err := m.MkConst(astTestSymbol(t, m, name), arraySort)
		astTestMust(t, err)
		arrays = append(arrays, x)
	}
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	for i := 0; i+1 < len(arrays); i++ {
		equal, err := m.MkEq(arrays[i], arrays[i+1])
		astTestMust(t, err)
		astTestMust(t, solver.Assert(equal))
	}
	phase := solver.kernel.smtParams.state.PhaseSelection
	status, err := smtSetupProfileCheck(solver)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("pure array result", status)
	}
	params := solver.kernel.smtParams
	if params.state.RelevancyLvl != 2 || params.state.PhaseSelection != phase {
		t.Fatal("setup_QF_AX() changed relevancy or phase", params.state.RelevancyLvl, params.state.PhaseSelection, phase)
	}
	if params.array.state.Mode != 1 || params.preprocess.state.NNFCNF {
		t.Fatal("setup_QF_AX() array mode or nnf_cnf", params.array.state.Mode, params.preprocess.state.NNFCNF)
	}
}
