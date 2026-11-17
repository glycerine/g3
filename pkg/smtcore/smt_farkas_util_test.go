package smtcore

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func smtFarkasFixture(t *testing.T) (*ast.ASTManager, *SMTFarkasUtil, ast.FamilyID, ast.ASTHandle) {
	t.Helper()
	m := astTestManager(t)
	context, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	astTestMust(t, err)
	globals, err := uparams.NewGlobalParamsService(m.Symbols, context, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	astTestMust(t, err)
	astTestMust(t, globals.InitializeSource())
	utility, err := NewSMTFarkasUtil(m, context, globals)
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, utility.Close())
		astTestMust(t, globals.Close())
		astTestMust(t, catalog.Close())
		astTestMust(t, context.Close())
	})
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	return m, utility, family, integer
}

func smtFarkasInt(t *testing.T, m *ast.ASTManager, value int64) ast.ASTHandle {
	t.Helper()
	result, err := m.ArithmeticNumeralRat(big.NewRat(value, 1), true)
	astTestMust(t, err)
	return result
}

func smtFarkasOp(t *testing.T, m *ast.ASTManager, family ast.FamilyID, op int32, args ...ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	result, err := m.MkFamilyApp(family, op, nil, args)
	astTestMust(t, err)
	return result
}

func smtFarkasConst(t *testing.T, m *ast.ASTManager, name string, sort ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	result, err := m.MkConst(astTestSymbol(t, m, name), sort)
	astTestMust(t, err)
	return result
}

func TestSMTFarkasEmptyIgnoredAndOwnedPremises(t *testing.T) {
	m, utility, family, integer := smtFarkasFixture(t)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer m.DelRef(&out)
	astTestMust(t, utility.Get(out))
	value, err := m.RefGet(out)
	astTestMust(t, err)
	if !m.IsFalse(value) {
		t.Fatal("empty Farkas consequence must be false")
	}
	truth, err := m.MkBoolVal(true)
	astTestMust(t, err)
	astTestMust(t, utility.Add(farkasTestQ(1, 1), truth))
	x := smtFarkasConst(t, m, "farkas-owned-x", integer)
	zero := smtFarkasInt(t, m, 0)
	constraint := smtFarkasOp(t, m, family, smtArithOpLE, x, zero)
	astTestMust(t, m.IncRef(constraint))
	defer m.DecRef(constraint)
	before, err := m.GetRefCount(constraint)
	astTestMust(t, err)
	astTestMust(t, utility.Add(farkasTestQ(0, 1), constraint))
	if len(utility.inequalities) != 0 {
		t.Fatal("zero coefficient or true premise was retained")
	}
	astTestMust(t, utility.Add(farkasTestQ(1, 1), constraint))
	after, err := m.GetRefCount(constraint)
	astTestMust(t, err)
	if after != before+1 {
		t.Fatalf("premise reference count = %d, want %d", after, before+1)
	}
	astTestMust(t, utility.Reset())
	after, err = m.GetRefCount(constraint)
	astTestMust(t, err)
	if after != before {
		t.Fatalf("reset reference count = %d, want %d", after, before)
	}
}

func TestSMTFarkasIntegerSignCorrection(t *testing.T) {
	m, utility, family, integer := smtFarkasFixture(t)
	x := smtFarkasConst(t, m, "farkas-sign-x", integer)
	y := smtFarkasConst(t, m, "farkas-sign-y", integer)
	strict := smtFarkasOp(t, m, family, smtArithOpLT, x, y)
	astTestMust(t, utility.Add(farkasTestQ(1, 1), strict))
	if op, err := m.GetDeclKind(utility.inequalities[0]); err != nil || op != smtArithOpLE {
		t.Fatalf("positive integer strict bound op = %d, err %v", op, err)
	}
	left, err := m.GetArg(utility.inequalities[0], 0)
	astTestMust(t, err)
	if op, err := m.GetDeclKind(left); err != nil || op != smtArithOpAdd {
		t.Fatalf("strict bound left op = %d, err %v", op, err)
	}
	astTestMust(t, utility.Reset())
	nonstrict := smtFarkasOp(t, m, family, smtArithOpLE, x, y)
	negated, err := m.MkNot(nonstrict)
	astTestMust(t, err)
	astTestMust(t, utility.Add(farkasTestQ(1, 1), negated))
	if op, err := m.GetDeclKind(utility.inequalities[0]); err != nil || op != smtArithOpGE {
		t.Fatalf("negative integer non-strict bound op = %d, err %v", op, err)
	}
	right, err := m.GetArg(utility.inequalities[0], 1)
	astTestMust(t, err)
	if op, err := m.GetDeclKind(right); err != nil || op != smtArithOpAdd {
		t.Fatalf("negative bound right op = %d, err %v", op, err)
	}
}

func TestSMTFarkasIntegerNormalizationAndConsequence(t *testing.T) {
	m, utility, family, integer := smtFarkasFixture(t)
	x := smtFarkasConst(t, m, "farkas-normal-x", integer)
	y := smtFarkasConst(t, m, "farkas-normal-y", integer)
	zero := smtFarkasInt(t, m, 0)
	astTestMust(t, utility.Add(farkasTestQ(1, 2), smtFarkasOp(t, m, family, smtArithOpLE, x, zero)))
	astTestMust(t, utility.Add(farkasTestQ(1, 3), smtFarkasOp(t, m, family, smtArithOpLE, y, zero)))
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer m.DelRef(&out)
	astTestMust(t, utility.Get(out))
	factor, err := utility.NormalizeFactor()
	astTestMust(t, err)
	asBig := func(r *numeral.Rational) *big.Rat {
		out := new(big.Rat)
		astTestMust(t, utility.q.BigRatOf(r, out))
		return out
	}
	if asBig(factor).Cmp(big.NewRat(6, 1)) != 0 {
		t.Fatalf("normalize factor = %s, want 6", asBig(factor))
	}
	if asBig(&utility.coefficients[0]).Cmp(big.NewRat(3, 1)) != 0 || asBig(&utility.coefficients[1]).Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatalf("normalized coefficients = %s, %s", asBig(&utility.coefficients[0]), asBig(&utility.coefficients[1]))
	}
	result, err := m.RefGet(out)
	astTestMust(t, err)
	if result.IsNull() || m.IsFalse(result) {
		t.Fatal("nonempty Farkas premises produced no consequence")
	}
}

func TestSMTFarkasSplitDisconnectedLiterals(t *testing.T) {
	m, utility, family, integer := smtFarkasFixture(t)
	x := smtFarkasConst(t, m, "farkas-split-x", integer)
	y := smtFarkasConst(t, m, "farkas-split-y", integer)
	zero := smtFarkasInt(t, m, 0)
	astTestMust(t, utility.Add(farkasTestQ(1, 1), smtFarkasOp(t, m, family, smtArithOpLE, x, zero)))
	astTestMust(t, utility.Add(farkasTestQ(1, 1), smtFarkasOp(t, m, family, smtArithOpLE, y, zero)))
	astTestMust(t, utility.SetSplitLiterals(true))
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer m.DelRef(&out)
	astTestMust(t, utility.Get(out))
	result, err := m.RefGet(out)
	astTestMust(t, err)
	if familyID, err := m.GetFamilyID(result); err != nil || familyID != m.BasicFamilyID() {
		t.Fatalf("split result family = %d, err %v", familyID, err)
	}
	if op, err := m.GetDeclKind(result); err != nil || op != ast.ASTOpOr {
		t.Fatalf("split result op = %d, err %v", op, err)
	}
	if count, err := m.GetNumArgs(result); err != nil || count != 2 {
		t.Fatalf("split result count = %d, err %v", count, err)
	}
}

// farkasTestQ is rational(n)/rational(d) for small n, d in lowest terms
// with d > 0: an inline rational that owns no cell.
func farkasTestQ(n, d int32) *numeral.Rational {
	return &numeral.Rational{Num: numeral.NewInt(n), Den: numeral.NewInt(d)}
}
