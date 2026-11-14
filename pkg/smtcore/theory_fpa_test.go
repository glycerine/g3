package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtFloatingPointSolver(t *testing.T) (*ast.ASTManager, ast.FamilyID, ast.ASTHandle, *SMTSolver) {
	t.Helper()
	m, modelConfig := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	family, err := m.RegisterFloatingPointPlugin()
	astTestMust(t, err)
	// A four-bit format keeps the focused SAT checks small while exercising the
	// same converter paths as the standard formats.
	sort, err := m.FloatingPointSort(2, 2)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "QF_FPBV", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	_, ok := solver.kernel.internalizer.theories[family].(*SMTFloatingPointTheory)
	if !ok {
		t.Fatal("QF_FPBV did not install the floating-point theory")
	}
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	return m, family, sort, solver
}

func TestSMTQFFPSetupInstallsFloatingPointAndBitVectorTheories(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	bvFamily, err := m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	fpFamily, err := m.RegisterFloatingPointPlugin()
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "QF_FP", SMTSolverConfig{
		Params:         modelConfig.Context,
		ModelEvaluator: modelConfig,
	})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	if _, ok := solver.kernel.internalizer.theories[bvFamily].(*SMTBitVectorTheory); !ok {
		t.Fatal("QF_FP did not install the bit-vector reduction theory")
	}
	if _, ok := solver.kernel.internalizer.theories[fpFamily].(*SMTFloatingPointTheory); !ok {
		t.Fatal("QF_FP did not install the floating-point theory")
	}
}

func TestSMTFloatingPointIEEEAndStructuralEqualities(t *testing.T) {
	t.Run("nan-ieee-self", func(t *testing.T) {
		m, family, sort, solver := smtFloatingPointSolver(t)
		nan, err := m.MkFPANaNOfSort(sort)
		astTestMust(t, err)
		equal, err := m.MkFamilyApp(family, ast.FPAOpEq, nil, []ast.ASTHandle{nan, nan})
		astTestMust(t, err)
		astTestMust(t, solver.Assert(equal))
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal("fp.eq NaN NaN accepted", status)
		}
	})

	t.Run("signed-zero-ieee", func(t *testing.T) {
		m, family, sort, solver := smtFloatingPointSolver(t)
		positive, err := m.MkFPAZeroOfSort(sort, false)
		astTestMust(t, err)
		negative, err := m.MkFPAZeroOfSort(sort, true)
		astTestMust(t, err)
		equal, err := m.MkFamilyApp(family, ast.FPAOpEq, nil, []ast.ASTHandle{positive, negative})
		astTestMust(t, err)
		distinct, err := m.MkNot(equal)
		astTestMust(t, err)
		astTestMust(t, solver.Assert(distinct))
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal("fp.eq distinguished signed zero", status)
		}
	})

	t.Run("signed-zero-structural", func(t *testing.T) {
		m, _, sort, solver := smtFloatingPointSolver(t)
		positive, err := m.MkFPAZeroOfSort(sort, false)
		astTestMust(t, err)
		negative, err := m.MkFPAZeroOfSort(sort, true)
		astTestMust(t, err)
		equal, err := m.MkEq(positive, negative)
		astTestMust(t, err)
		astTestMust(t, solver.Assert(equal))
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal("structural equality merged signed zero", status)
		}
	})
}

func TestSMTFloatingPointWrappedConstantAndScopes(t *testing.T) {
	m, _, sort, solver := smtFloatingPointSolver(t)
	x, err := m.MkConst(astTestSymbol(t, m, "smt-fpa-x"), sort)
	astTestMust(t, err)
	positive, err := m.MkFPAZeroOfSort(sort, false)
	astTestMust(t, err)
	negative, err := m.MkFPAZeroOfSort(sort, true)
	astTestMust(t, err)
	positiveEquality, err := m.MkEq(x, positive)
	astTestMust(t, err)
	negativeEquality, err := m.MkEq(x, negative)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(positiveEquality))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("wrapped FP constant equality", status)
	}
	astTestMust(t, solver.Push())
	astTestMust(t, solver.Assert(negativeEquality))
	status, err = solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("wrapped FP constant accepted incompatible encodings", status)
	}
	astTestMust(t, solver.Pop(1))
	status, err = solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("floating-point conversion cache did not roll back", status)
	}
}

func TestSMTFloatingPointConversionTermsAndArithmetic(t *testing.T) {
	t.Run("to-ieee-bv", func(t *testing.T) {
		m, family, sort, solver := smtFloatingPointSolver(t)
		zero, err := m.MkFPAZeroOfSort(sort, false)
		astTestMust(t, err)
		encoded, err := m.MkFamilyApp(family, ast.FPAOpToIEEEBV, nil, []ast.ASTHandle{zero})
		astTestMust(t, err)
		one, err := m.BitVectorNumeralUint64(1, 4)
		astTestMust(t, err)
		equal, err := m.MkEq(encoded, one)
		astTestMust(t, err)
		astTestMust(t, solver.Assert(equal))
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal("positive zero received nonzero IEEE encoding", status)
		}
	})

	t.Run("zero-addition", func(t *testing.T) {
		m, family, sort, solver := smtFloatingPointSolver(t)
		zero, err := m.MkFPAZeroOfSort(sort, false)
		astTestMust(t, err)
		rne, err := m.MkFamilyApp(family, ast.FPAOpRMNearestEven, nil, nil)
		astTestMust(t, err)
		sum, err := m.MkFamilyApp(family, ast.FPAOpAdd, nil, []ast.ASTHandle{rne, zero, zero})
		astTestMust(t, err)
		equal, err := m.MkFamilyApp(family, ast.FPAOpEq, nil, []ast.ASTHandle{sum, zero})
		astTestMust(t, err)
		negated, err := m.MkNot(equal)
		astTestMust(t, err)
		astTestMust(t, solver.Assert(negated))
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal("positive zero addition was not reduced", status)
		}
	})
}

func TestSMTFloatingPointModelReconstructsWrappedConstants(t *testing.T) {
	m, family, sort, solver := smtFloatingPointSolver(t)
	astTestMust(t, solver.SetProduceModels(true))
	x, err := m.MkConst(astTestSymbol(t, m, "smt-fpa-model-x"), sort)
	astTestMust(t, err)
	negative, err := m.MkFPAZeroOfSort(sort, true)
	astTestMust(t, err)
	equality, err := m.MkEq(x, negative)
	astTestMust(t, err)
	roundingSort, err := m.RoundingModeSort()
	astTestMust(t, err)
	rounding, err := m.MkConst(astTestSymbol(t, m, "smt-fpa-model-rm"), roundingSort)
	astTestMust(t, err)
	towardNegative, err := m.MkFamilyApp(family, ast.FPAOpRMTowardNegative, nil, nil)
	astTestMust(t, err)
	roundingEquality, err := m.MkEq(rounding, towardNegative)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(equality))
	astTestMust(t, solver.Assert(roundingEquality))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("floating-point model constraints", status)
	}
	generated, err := solver.AcquireModel()
	astTestMust(t, err)
	if generated == nil {
		t.Fatal("floating-point solver omitted model")
	}
	t.Cleanup(func() { astTestMust(t, generated.DecRef()) })
	xDeclaration, err := m.GetDecl(x)
	astTestMust(t, err)
	xValue, err := generated.ConstantInterpretation(xDeclaration)
	astTestMust(t, err)
	if op, e := m.GetDeclKind(xValue); e != nil || op != ast.FPAOpNegativeZero {
		t.Fatal("floating-point model value", xValue, op, e)
	}
	roundingDeclaration, err := m.GetDecl(rounding)
	astTestMust(t, err)
	roundingValue, err := generated.ConstantInterpretation(roundingDeclaration)
	astTestMust(t, err)
	if op, e := m.GetDeclKind(roundingValue); e != nil || op != ast.FPAOpRMTowardNegative {
		t.Fatal("rounding-mode model value", roundingValue, op, e)
	}
}

func TestSMTFloatingPointModelFinalizesConvertedFunction(t *testing.T) {
	m, _, sort, solver := smtFloatingPointSolver(t)
	astTestMust(t, solver.SetProduceModels(true))
	x, err := m.MkConst(astTestSymbol(t, m, "smt-fpa-model-uf-x"), sort)
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "smt-fpa-model-uf"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	application, err := m.MkApp(function, []ast.ASTHandle{x})
	astTestMust(t, err)
	positive, err := m.MkFPAZeroOfSort(sort, false)
	astTestMust(t, err)
	negative, err := m.MkFPAZeroOfSort(sort, true)
	astTestMust(t, err)
	xEquality, err := m.MkEq(x, positive)
	astTestMust(t, err)
	resultEquality, err := m.MkEq(application, negative)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(xEquality))
	astTestMust(t, solver.Assert(resultEquality))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("floating-point UF model constraints", status)
	}
	generated, err := solver.AcquireModel()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, generated.DecRef()) })
	interpretation, err := generated.FunctionInterpretation(function)
	astTestMust(t, err)
	if interpretation == nil {
		t.Fatal("floating-point UF interpretation was not restored")
	}
	if interpretation.NumEntries() == 0 {
		t.Fatal("floating-point UF interpretation omitted reached application")
	}
}

// AUDIT.md smtcore_11-F2: theory_fpa::mk_value returns the owner only for
// is_rm_numeral/is_numeral; an (fp b e s) class root goes through the
// fpa_value_proc and is canonicalized (here to +zero).
func TestSMTFloatingPointModelCanonicalizesFPTripleRoot(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	family, err := m.RegisterFloatingPointPlugin()
	astTestMust(t, err)
	sort, err := m.FloatingPointSort(2, 2)
	astTestMust(t, err)
	// Relevancy 0 keeps the converted (fp ...) term as the class root.
	p, err := modelConfig.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, modelConfig.Context.Del(&p)) })
	key := "relevancy"
	astTestMust(t, modelConfig.Context.SetUIntName(&p, &key, 0))
	solver, err := NewSMTSolver(m, &p, "QF_FPBV", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.SetProduceModels(true))
	x, err := m.MkConst(astTestSymbol(t, m, "smt-fpa-triple-x"), sort)
	astTestMust(t, err)
	// (= x ((_ to_fp 2 2) rm 0.0)): the converter leaves an (fp ...) triple
	// of numerals as the class root, as for the QF_FP script.
	roundingSort, err := m.RoundingModeSort()
	astTestMust(t, err)
	rounding, err := m.MkConst(astTestSymbol(t, m, "smt-fpa-triple-rm"), roundingSort)
	astTestMust(t, err)
	zero, err := m.ArithmeticNumeralRat(new(big.Rat), false)
	astTestMust(t, err)
	eParam, err := m.NewParameterInt(2)
	astTestMust(t, err)
	sParam, err := m.NewParameterInt(2)
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, m.DelParameter(&sParam))
		astTestMust(t, m.DelParameter(&eParam))
	})
	triple, err := m.MkFamilyApp(family, ast.FPAOpToFP, []ast.ASTParameter{eParam, sParam}, []ast.ASTHandle{rounding, zero})
	astTestMust(t, err)
	equality, err := m.MkEq(x, triple)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(equality))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("fp triple constraint", status)
	}
	generated, err := solver.AcquireModel()
	astTestMust(t, err)
	if generated == nil {
		t.Fatal("floating-point solver omitted model")
	}
	t.Cleanup(func() { astTestMust(t, generated.DecRef()) })
	declaration, err := m.GetDecl(x)
	astTestMust(t, err)
	value, err := generated.ConstantInterpretation(declaration)
	astTestMust(t, err)
	if op, e := m.GetDeclKind(value); e != nil || op != ast.FPAOpPositiveZero {
		t.Fatal("fp triple model value was not canonicalized", value, op, e)
	}
}
