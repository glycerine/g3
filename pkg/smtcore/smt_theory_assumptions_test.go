package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtAssumptionTestTheory struct {
	*smtInternalizerTestTheory
	assumption            ast.ASTHandle
	addCalls, validations int
	input                 []ast.ASTHandle
	core                  []ast.ASTHandle
	validationLevel       uint32
	status                lbool.LBool
	validationError       error
}

func (t *smtAssumptionTestTheory) AddTheoryAssumptions(input []ast.ASTHandle) ([]ast.ASTHandle, error) {
	t.addCalls++
	t.input = append([]ast.ASTHandle(nil), input...)
	return append(input, t.assumption), nil
}

func (t *smtAssumptionTestTheory) ValidateUnsatCore(core []ast.ASTHandle) (lbool.LBool, error) {
	t.validations++
	t.core = append([]ast.ASTHandle(nil), core...)
	t.validationLevel = t.state.scopeLevel
	return t.status, t.validationError
}

func smtAssumptionTestFixture(t *testing.T, m *ast.ASTManager, k *SMTKernel, assumption ast.ASTHandle) *smtAssumptionTestTheory {
	t.Helper()
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	base, err := NewSMTTheoryBase(family)
	astTestMust(t, err)
	theory := &smtAssumptionTestTheory{smtInternalizerTestTheory: &smtInternalizerTestTheory{SMTTheoryBase: base}, assumption: assumption, status: lbool.Undef}
	astTestMust(t, k.internalizer.RegisterTheory(theory))
	return theory
}

func TestSMTTheoryAssumptionsOrdinaryCheckAndValidation(t *testing.T) {
	for _, status := range []lbool.LBool{lbool.Undef, lbool.False, lbool.True} {
		m := astTestManager(t)
		k := smtTestKernel(t, m)
		a := smtRelevancyBool(t, m, "theory-assume-a")
		notA, err := m.MkNot(a)
		astTestMust(t, err)
		astTestPin(t, m, notA)
		theory := smtAssumptionTestFixture(t, m, k, notA)
		theory.status = status
		astTestMust(t, k.Assert(a, ast.ASTHandle{}))
		result, err := k.Check()
		astTestMust(t, err)
		want := lbool.False
		if status == lbool.Undef {
			want = lbool.Undef
		}
		if result != want || theory.addCalls != 1 || theory.validations != 1 || theory.validationLevel != 0 {
			t.Fatal("ordinary check omitted theory assumptions or post-pop validation", status, result, theory.addCalls, theory.validations, theory.validationLevel)
		}
		if len(theory.core) != 1 || theory.core[0] != notA || len(k.UnsatCore()) != 1 || k.UnsatCore()[0] != notA {
			t.Fatal("theory assumption missing from the extracted core", theory.core, k.UnsatCore())
		}
		if k.internalizer.scopeLevel != 0 || k.Statistics().NumChecks != 1 || k.internalizer.booleanData[k.internalizer.boolVar.get(a)].Assumption {
			t.Fatal("theory assumptions leaked a scope/flag or double-counted the check")
		}
	}
}

func TestSMTTheoryAssumptionsExternalOrderRollbackAndErrors(t *testing.T) {
	m := astTestManager(t)
	k := smtTestKernel(t, m)
	a := smtRelevancyBool(t, m, "theory-external-a")
	b := smtRelevancyBool(t, m, "theory-external-b")
	astTestPin(t, m, b)
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	astTestPin(t, m, notA)
	theory := smtAssumptionTestFixture(t, m, k, notA)
	astTestMust(t, k.Assert(a, ast.ASTHandle{}))
	external := []ast.ASTHandle{b}
	result, err := k.CheckAssumptions(external)
	astTestMust(t, err)
	if result != lbool.Undef || len(theory.input) != 1 || theory.input[0] != b || len(external) != 1 || external[0] != b {
		t.Fatal("external assumptions were lost, mutated or added twice", result, theory.input, external)
	}
	sentinel := errors.New("theory validation failed")
	theory.validationError = sentinel
	result, err = k.CheckAssumptions(external)
	if result != lbool.Undef || !errors.Is(err, sentinel) || k.internalizer.scopeLevel != 0 || k.assumptions != nil {
		t.Fatal("validation error lost cleanup or returned a definitive result", result, err)
	}
	theory.validationError = nil
	theory.assumption = b
	before := theory.validations
	result, err = k.Check()
	astTestMust(t, err)
	if result != lbool.True || theory.validations != before || len(k.UnsatCore()) != 0 {
		t.Fatal("satisfiable check validated a stale core or retained old assumptions", result, theory.validations, before)
	}
}

func TestSMTTheoryAssumptionsSetupAndCheckCountsDelegatedCheck(t *testing.T) {
	m := astTestManager(t)
	k := smtTestKernel(t, m)
	a := smtRelevancyBool(t, m, "setup-theory-assumption")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	astTestPin(t, m, notA)
	theory := smtAssumptionTestFixture(t, m, k, notA)
	astTestMust(t, k.Assert(a, ast.ASTHandle{}))
	result, err := k.SetupAndCheck()
	astTestMust(t, err)
	if result != lbool.Undef || theory.addCalls != 1 || theory.validations != 1 || k.Statistics().NumChecks != 1 || k.internalizer.scopeLevel != 0 {
		t.Fatal("setup/check did not delegate exactly once with its collected theory assumptions", result, theory.addCalls, theory.validations, k.Statistics())
	}
}

func TestSMTTheoryAssumptionsTrueInputStillValidatesEmptyCore(t *testing.T) {
	m := astTestManager(t)
	k := smtTestKernel(t, m)
	truth, err := m.MkBoolVal(true)
	astTestMust(t, err)
	theory := smtAssumptionTestFixture(t, m, k, truth)
	a := smtRelevancyBool(t, m, "empty-core-a")
	b := smtRelevancyBool(t, m, "empty-core-b")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	astTestPin(t, m, notA)
	notB, err := m.MkNot(b)
	astTestMust(t, err)
	astTestPin(t, m, notB)
	for _, pair := range [][2]ast.ASTHandle{{a, b}, {notA, b}, {a, notB}, {notA, notB}} {
		clause, err := m.MkOr(pair[0], pair[1])
		astTestMust(t, err)
		astTestMust(t, k.Assert(clause, ast.ASTHandle{}))
	}
	result, err := k.Check()
	astTestMust(t, err)
	if result != lbool.Undef || theory.validations != 1 || len(theory.core) != 0 || theory.validationLevel != 0 {
		t.Fatal("true-only assumption scope did not validate its empty core", result, theory.validations, theory.core, theory.validationLevel)
	}
}
