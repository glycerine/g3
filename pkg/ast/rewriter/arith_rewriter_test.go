package rewriter

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func arithRewriterNumeral(t *testing.T, m *ast.ASTManager, text string, integer bool) ast.ASTHandle {
	t.Helper()
	service := m.NumeralService()
	n, err := service.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, service.Del(&n)) }()
	astTestMust(t, service.SetString(&n, text))
	h, err := m.ArithmeticNumeral(&n, integer)
	astTestMust(t, err)
	astTestPin(t, m, h)
	return h
}

func arithRewriterCore(t *testing.T, r *ArithRewriter, kind int32, args ...ast.ASTHandle) (RewriteStatus, ast.ASTHandle) {
	t.Helper()
	app, err := r.manager.MkFamilyApp(r.family, kind, nil, args)
	astTestMust(t, err)
	decl, err := r.manager.GetDecl(app)
	astTestMust(t, err)
	out := astTestPin(t, r.manager, app)
	status, err := r.MkAppCore(decl, ast.SliceArguments(args), out)
	astTestMust(t, err)
	result, err := r.manager.RefGet(out)
	astTestMust(t, err)
	return status, result
}

func arithRewriterWantNumeral(t *testing.T, m *ast.ASTManager, h ast.ASTHandle, text string, integer bool) {
	t.Helper()
	got, gotInteger, matched, err := m.ArithmeticNumeralText(h)
	astTestMust(t, err)
	if !matched || got != text || gotInteger != integer {
		t.Fatal("arithmetic numeral", got, gotInteger, matched, "want", text, integer)
	}
}

func arithRewriterWantOperation(t *testing.T, r *ArithRewriter, h ast.ASTHandle, kind int32, args ...ast.ASTHandle) {
	t.Helper()
	gotKind, gotArgs, matched, err := r.operation(h, kind)
	astTestMust(t, err)
	if !matched || gotKind != kind || len(gotArgs) != len(args) {
		t.Fatal("arithmetic operation", gotKind, len(gotArgs), matched, "want", kind, len(args))
	}
	for i := range args {
		if gotArgs[i] != args[i] {
			t.Fatal("arithmetic argument", i, gotArgs[i], args[i])
		}
	}
}

func TestArithRewriterRationalScalarCores(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	i2 := arithRewriterNumeral(t, m, "2", true)
	i3 := arithRewriterNumeral(t, m, "3", true)
	iNeg3 := arithRewriterNumeral(t, m, "-3", true)
	iNeg2 := arithRewriterNumeral(t, m, "-2", true)
	r2 := arithRewriterNumeral(t, m, "2", false)
	r3 := arithRewriterNumeral(t, m, "3", false)
	rHalf := arithRewriterNumeral(t, m, "1/2", false)
	rNegThreeHalves := arithRewriterNumeral(t, m, "-3/2", false)

	for _, row := range []struct {
		kind        int32
		args        []ast.ASTHandle
		want        string
		wantInteger bool
		wantStatus  RewriteStatus
	}{
		{arithOpAdd, []ast.ASTHandle{i2, i3}, "5", true, RewriteDone},
		{arithOpMul, []ast.ASTHandle{rHalf, r3}, "3/2", false, RewriteDone},
		{arithOpDiv, []ast.ASTHandle{r3, r2}, "3/2", false, RewriteDone},
		{arithOpIDiv, []ast.ASTHandle{iNeg3, i2}, "-2", true, RewriteDone},
		{arithOpIDiv, []ast.ASTHandle{i3, iNeg2}, "-1", true, RewriteDone},
		{arithOpMod, []ast.ASTHandle{iNeg3, i2}, "1", true, RewriteDone},
		{arithOpMod, []ast.ASTHandle{i3, iNeg2}, "1", true, RewriteDone},
		{arithOpRem, []ast.ASTHandle{i3, iNeg2}, "-1", true, RewriteDone},
		{arithOpToInt, []ast.ASTHandle{rNegThreeHalves}, "-2", true, RewriteDone},
		{arithOpToReal, []ast.ASTHandle{i3}, "3", false, RewriteDone},
		{arithOpPower, []ast.ASTHandle{i2, i3}, "8", true, RewriteDone},
		{arithOpPower, []ast.ASTHandle{r2, iNeg2}, "1/4", false, RewriteDone},
	} {
		status, result := arithRewriterCore(t, r, row.kind, row.args...)
		if status != row.wantStatus {
			t.Fatal("arithmetic status", row.kind, status, row.wantStatus)
		}
		arithRewriterWantNumeral(t, m, result, row.want, row.wantInteger)
	}

	status, result := arithRewriterCore(t, r, arithOpLE, i2, i3)
	if status != RewriteDone || !m.IsTrue(result) {
		t.Fatal("numeric comparison", status, result)
	}
	status, result = arithRewriterCore(t, r, arithOpIsInt, rHalf)
	if status != RewriteDone || !m.IsFalse(result) {
		t.Fatal("numeric is-int", status, result)
	}
	out := astTestPin(t, m, result)
	status, err = r.MkEqCore(i2, i3, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || !m.IsFalse(result) {
		t.Fatal("numeric equality", status, result)
	}
}

func TestArithRewriterStructuralScalarRules(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	iSort, err := m.MkFamilySort(r.family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "arith-rewriter-x"), iSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	i2 := arithRewriterNumeral(t, m, "2", true)

	status, result := arithRewriterCore(t, r, arithOpLT, x, i2)
	if status != Rewrite2 {
		t.Fatal("less-than status", status)
	}
	if yes, _, err := m.IsNot(result); err != nil || !yes {
		t.Fatal("less-than source negated comparison", yes, err)
	}
	status, result = arithRewriterCore(t, r, arithOpAbs, x)
	if status != Rewrite2 {
		t.Fatal("absolute-value status", status)
	}
	if kind, err := m.GetDeclKind(result); err != nil || kind != ast.ASTOpIte {
		t.Fatal("absolute-value ITE", kind, err)
	}
	toReal, err := m.MkFamilyApp(r.family, arithOpToReal, nil, []ast.ASTHandle{x})
	astTestMust(t, err)
	astTestPin(t, m, toReal)
	status, result = arithRewriterCore(t, r, arithOpToInt, toReal)
	if status != RewriteDone || result != x {
		t.Fatal("to-int/to-real cancellation", status, result, x)
	}
	status, result = arithRewriterCore(t, r, arithOpIsInt, toReal)
	if status != RewriteDone || !m.IsTrue(result) {
		t.Fatal("to-real is integral", status, result)
	}
}

func TestArithRewriterSymbolicModRemAndCasts(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	iSort, err := m.MkFamilySort(r.family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "arith-symbolic-x"), iSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	y, err := m.MkConst(astTestSymbol(t, m, "arith-symbolic-y"), iSort)
	astTestMust(t, err)
	astTestPin(t, m, y)
	i1 := arithRewriterNumeral(t, m, "1", true)
	i2 := arithRewriterNumeral(t, m, "2", true)
	iNeg2 := arithRewriterNumeral(t, m, "-2", true)
	r1 := arithRewriterNumeral(t, m, "1", false)

	status, result := arithRewriterCore(t, r, arithOpMod, x, i1)
	if status != RewriteDone {
		t.Fatal("modulo one status", status)
	}
	arithRewriterWantNumeral(t, m, result, "0", true)

	status, result = arithRewriterCore(t, r, arithOpMod, x, x)
	if status != RewriteDone {
		t.Fatal("self modulo status", status)
	}
	if kind, e := m.GetDeclKind(result); e != nil || kind != ast.ASTOpIte {
		t.Fatal("self modulo ITE", kind, e)
	}

	innerMod, err := r.app(arithOpMod, x, i2)
	astTestMust(t, err)
	astTestPin(t, m, innerMod)
	status, result = arithRewriterCore(t, r, arithOpMod, innerMod, i2)
	if status != RewriteDone || result != innerMod {
		t.Fatal("idempotent modulo", status, result, innerMod)
	}

	sum, err := r.app(arithOpAdd, x, i1)
	astTestMust(t, err)
	astTestPin(t, m, sum)
	status, result = arithRewriterCore(t, r, arithOpMod, sum, i2)
	if status != Rewrite3 {
		t.Fatal("modulo propagation status", status)
	}
	_, outerArgs, matched, err := r.operation(result, arithOpMod)
	astTestMust(t, err)
	if !matched || len(outerArgs) != 2 || outerArgs[1] != i2 {
		t.Fatal("modulo propagation outer", matched, len(outerArgs))
	}
	_, addArgs, matched, err := r.operation(outerArgs[0], arithOpAdd)
	astTestMust(t, err)
	if !matched || len(addArgs) != 2 {
		t.Fatal("modulo propagation sum", matched, len(addArgs))
	}
	for _, arg := range addArgs {
		_, modArgs, yes, e := r.operation(arg, arithOpMod)
		astTestMust(t, e)
		if !yes || len(modArgs) != 2 || modArgs[1] != i2 {
			t.Fatal("modulo propagation child", yes, len(modArgs))
		}
	}

	status, result = arithRewriterCore(t, r, arithOpRem, x, i2)
	if status != Rewrite1 {
		t.Fatal("positive symbolic remainder", status)
	}
	arithRewriterWantOperation(t, r, result, arithOpMod, x, i2)
	status, result = arithRewriterCore(t, r, arithOpRem, x, iNeg2)
	if status != Rewrite2 {
		t.Fatal("negative symbolic remainder", status)
	}
	_, negativeArgs, matched, err := r.operation(result, arithOpUMinus)
	astTestMust(t, err)
	if !matched || len(negativeArgs) != 1 {
		t.Fatal("negative symbolic remainder shape", matched, len(negativeArgs))
	}
	arithRewriterWantOperation(t, r, negativeArgs[0], arithOpMod, x, iNeg2)

	toRealX, err := r.app(arithOpToReal, x)
	astTestMust(t, err)
	astTestPin(t, m, toRealX)
	realSum, err := r.app(arithOpAdd, r1, toRealX)
	astTestMust(t, err)
	astTestPin(t, m, realSum)
	status, result = arithRewriterCore(t, r, arithOpToInt, realSum)
	if status != Rewrite1 {
		t.Fatal("to-int distribution status", status)
	}
	arithRewriterWantOperation(t, r, result, arithOpAdd, i1, x)

	status, result = arithRewriterCore(t, r, arithOpToReal, sum)
	if status != Rewrite2 {
		t.Fatal("to-real distribution status", status)
	}
	_, realArgs, matched, err := r.operation(result, arithOpAdd)
	astTestMust(t, err)
	if !matched || len(realArgs) != 2 {
		t.Fatal("to-real distribution sum", matched, len(realArgs))
	}
	arithRewriterWantOperation(t, r, realArgs[0], arithOpToReal, x)
	arithRewriterWantOperation(t, r, realArgs[1], arithOpToReal, i1)

	key := "elim_rem"
	astTestMust(t, c.SetBoolName(local, &key, true))
	eliminating, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, eliminating.Close()) })
	status, result = arithRewriterCore(t, eliminating, arithOpRem, x, y)
	if status != Rewrite3 {
		t.Fatal("remainder elimination status", status)
	}
	if kind, e := m.GetDeclKind(result); e != nil || kind != ast.ASTOpIte {
		t.Fatal("remainder elimination ITE", kind, e)
	}
}

func TestArithRewriterPolynomialAddMulSubAndNegation(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	key := "mul_to_power"
	astTestMust(t, c.SetBoolName(local, &key, true))
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	iSort, err := m.MkFamilySort(r.family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "arith-poly-x"), iSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	y, err := m.MkConst(astTestSymbol(t, m, "arith-poly-y"), iSort)
	astTestMust(t, err)
	astTestPin(t, m, y)
	i0 := arithRewriterNumeral(t, m, "0", true)
	i2 := arithRewriterNumeral(t, m, "2", true)
	i3 := arithRewriterNumeral(t, m, "3", true)

	status, result := arithRewriterCore(t, r, arithOpAdd, x, x)
	if status != RewriteDone {
		t.Fatal("combine like monomials status", status)
	}
	arithRewriterWantOperation(t, r, result, arithOpMul, i2, x)

	nested, err := r.app(arithOpAdd, y, x)
	astTestMust(t, err)
	astTestPin(t, m, nested)
	status, result = arithRewriterCore(t, r, arithOpAdd, x, nested)
	if status != RewriteDone {
		t.Fatal("flatten addition status", status)
	}
	_, sumArgs, matched, err := r.operation(result, arithOpAdd)
	astTestMust(t, err)
	if !matched || len(sumArgs) != 2 {
		t.Fatal("flattened addition", matched, len(sumArgs))
	}
	foundX, foundY := false, false
	for _, arg := range sumArgs {
		if arg == y {
			foundY = true
			continue
		}
		_, productArgs, yes, e := r.operation(arg, arithOpMul)
		astTestMust(t, e)
		foundX = foundX || yes && len(productArgs) == 2 && productArgs[0] == i2 && productArgs[1] == x
	}
	if !foundX || !foundY {
		t.Fatal("flattened addition terms", foundX, foundY)
	}

	status, result = arithRewriterCore(t, r, arithOpMul, x, x)
	if status != RewriteDone {
		t.Fatal("combine repeated factor status", status)
	}
	arithRewriterWantOperation(t, r, result, arithOpPower, x, i2)

	innerProduct, err := r.app(arithOpMul, i3, x)
	astTestMust(t, err)
	astTestPin(t, m, innerProduct)
	status, result = arithRewriterCore(t, r, arithOpMul, i2, innerProduct)
	if status != RewriteDone {
		t.Fatal("flatten product status", status)
	}
	_, productArgs, matched, err := r.operation(result, arithOpMul)
	astTestMust(t, err)
	if !matched || len(productArgs) != 2 || productArgs[1] != x {
		t.Fatal("flattened product", matched, len(productArgs))
	}
	arithRewriterWantNumeral(t, m, productArgs[0], "6", true)

	status, result = arithRewriterCore(t, r, arithOpUMinus, x)
	if status != Rewrite1 {
		t.Fatal("symbolic unary minus status", status)
	}
	_, negativeArgs, matched, err := r.operation(result, arithOpMul)
	astTestMust(t, err)
	if !matched || len(negativeArgs) != 2 || negativeArgs[1] != x {
		t.Fatal("symbolic unary minus", matched, len(negativeArgs))
	}
	arithRewriterWantNumeral(t, m, negativeArgs[0], "-1", true)

	status, result = arithRewriterCore(t, r, arithOpSub, x, y, i0)
	if status != Rewrite2 {
		t.Fatal("subtraction status", status)
	}
	_, subtractionArgs, matched, err := r.operation(result, arithOpAdd)
	astTestMust(t, err)
	if !matched || len(subtractionArgs) != 2 || subtractionArgs[0] != x {
		t.Fatal("subtraction addition", matched, len(subtractionArgs))
	}
	_, negativeArgs, matched, err = r.operation(subtractionArgs[1], arithOpMul)
	astTestMust(t, err)
	if !matched || len(negativeArgs) != 2 || negativeArgs[1] != y {
		t.Fatal("subtraction negative", matched, len(negativeArgs))
	}
	arithRewriterWantNumeral(t, m, negativeArgs[0], "-1", true)

	key = "flat"
	astTestMust(t, c.SetBoolName(local, &key, false))
	nonflat, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, nonflat.Close()) })
	status, _ = arithRewriterCore(t, nonflat, arithOpAdd, x, nested)
	if status != RewriteFailed {
		t.Fatal("disabled polynomial flattening", status)
	}
	key = "flat"
	astTestMust(t, c.SetBoolName(local, &key, true))
	key = "mul_to_power"
	astTestMust(t, c.SetBoolName(local, &key, false))
	withoutPowers, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, withoutPowers.Close()) })
	status, _ = arithRewriterCore(t, withoutPowers, arithOpMul, x, x)
	if status != RewriteFailed {
		t.Fatal("disabled multiplication-to-power", status)
	}
}

func TestArithRewriterPolynomialCancellation(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	iSort, err := m.MkFamilySort(r.family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "arith-cancel-x"), iSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	y, err := m.MkConst(astTestSymbol(t, m, "arith-cancel-y"), iSort)
	astTestMust(t, err)
	astTestPin(t, m, y)
	z, err := m.MkConst(astTestSymbol(t, m, "arith-cancel-z"), iSort)
	astTestMust(t, err)
	astTestPin(t, m, z)
	i2 := arithRewriterNumeral(t, m, "2", true)
	i5 := arithRewriterNumeral(t, m, "5", true)
	left, err := r.app(arithOpAdd, x, i2)
	astTestMust(t, err)
	astTestPin(t, m, left)
	right, err := r.app(arithOpAdd, x, i5)
	astTestMust(t, err)
	astTestPin(t, m, right)

	status, result := arithRewriterCore(t, r, arithOpLE, left, right)
	if status != RewriteDone || !m.IsTrue(result) {
		t.Fatal("inequality cancellation", status, result)
	}

	equalityOut := astTestPin(t, m, left)
	status, err = r.MkEqCore(left, right, equalityOut)
	astTestMust(t, err)
	result, err = m.RefGet(equalityOut)
	astTestMust(t, err)
	if status != RewriteDone || !m.IsFalse(result) {
		t.Fatal("equality cancellation", status, result)
	}

	left, err = r.app(arithOpAdd, x, y)
	astTestMust(t, err)
	astTestPin(t, m, left)
	right, err = r.app(arithOpAdd, x, z)
	astTestMust(t, err)
	astTestPin(t, m, right)
	status, result = arithRewriterCore(t, r, arithOpGE, left, right)
	if status != RewriteDone {
		t.Fatal("symbolic cancellation status", status)
	}
	arithRewriterWantOperation(t, r, result, arithOpGE, y, z)

	key := "arith_lhs"
	astTestMust(t, c.SetBoolName(local, &key, true))
	lhsRewriter, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, lhsRewriter.Close()) })
	left, err = lhsRewriter.app(arithOpAdd, x, i2)
	astTestMust(t, err)
	astTestPin(t, m, left)
	right, err = lhsRewriter.app(arithOpAdd, y, i5)
	astTestMust(t, err)
	astTestPin(t, m, right)
	status, result = arithRewriterCore(t, lhsRewriter, arithOpLE, left, right)
	if status != RewriteDone {
		t.Fatal("move-to-lhs status", status)
	}
	_, comparisonArgs, matched, err := lhsRewriter.operation(result, arithOpLE)
	astTestMust(t, err)
	if !matched || len(comparisonArgs) != 2 {
		t.Fatal("move-to-lhs comparison", matched, len(comparisonArgs))
	}
	arithRewriterWantNumeral(t, m, comparisonArgs[1], "3", true)
	_, lhsArgs, matched, err := lhsRewriter.operation(comparisonArgs[0], arithOpAdd)
	astTestMust(t, err)
	if !matched || len(lhsArgs) != 2 || lhsArgs[0] != x {
		t.Fatal("move-to-lhs polynomial", matched, len(lhsArgs))
	}
	_, negativeY, matched, err := lhsRewriter.operation(lhsArgs[1], arithOpMul)
	astTestMust(t, err)
	if !matched || len(negativeY) != 2 || negativeY[1] != y {
		t.Fatal("move-to-lhs negative term", matched, len(negativeY))
	}
	arithRewriterWantNumeral(t, m, negativeY[0], "-1", true)
}

func TestArithRewriterCoefficientAndModuloBounds(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	iSort, err := m.MkFamilySort(r.family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "arith-bound-x"), iSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	i0 := arithRewriterNumeral(t, m, "0", true)
	i2 := arithRewriterNumeral(t, m, "2", true)
	i4 := arithRewriterNumeral(t, m, "4", true)
	i5 := arithRewriterNumeral(t, m, "5", true)
	iNeg2 := arithRewriterNumeral(t, m, "-2", true)
	iNeg3 := arithRewriterNumeral(t, m, "-3", true)
	twice, err := r.app(arithOpMul, i2, x)
	astTestMust(t, err)
	astTestPin(t, m, twice)

	status, result := arithRewriterCore(t, r, arithOpLE, twice, i5)
	if status != RewriteDone {
		t.Fatal("positive coefficient bound status", status)
	}
	arithRewriterWantOperation(t, r, result, arithOpLE, x, i2)

	negativeTwice, err := r.app(arithOpMul, iNeg2, x)
	astTestMust(t, err)
	astTestPin(t, m, negativeTwice)
	status, result = arithRewriterCore(t, r, arithOpGE, negativeTwice, i5)
	if status != RewriteDone {
		t.Fatal("negative coefficient bound status", status)
	}
	arithRewriterWantOperation(t, r, result, arithOpLE, x, iNeg3)

	equalityOut := astTestPin(t, m, twice)
	status, err = r.MkEqCore(twice, i5, equalityOut)
	astTestMust(t, err)
	result, err = m.RefGet(equalityOut)
	astTestMust(t, err)
	if status != RewriteDone || !m.IsFalse(result) {
		t.Fatal("non-integral coefficient equality", status, result)
	}

	mod, err := r.app(arithOpMod, x, i5)
	astTestMust(t, err)
	astTestPin(t, m, mod)
	status, result = arithRewriterCore(t, r, arithOpLE, mod, i4)
	if status != RewriteDone || !m.IsTrue(result) {
		t.Fatal("modulo upper bound", status, result)
	}
	status, result = arithRewriterCore(t, r, arithOpGE, mod, i0)
	if status != RewriteDone || !m.IsTrue(result) {
		t.Fatal("modulo lower bound", status, result)
	}
	equalityOut = astTestPin(t, m, mod)
	status, err = r.MkEqCore(mod, i5, equalityOut)
	astTestMust(t, err)
	result, err = m.RefGet(equalityOut)
	astTestMust(t, err)
	if status != RewriteDone || !m.IsFalse(result) {
		t.Fatal("modulo excluded equality", status, result)
	}
}

func TestArithRewriterStructuralPowerRules(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	realSort, err := m.MkFamilySort(r.family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "arith-power-x"), realSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	r2 := arithRewriterNumeral(t, m, "2", false)
	r3 := arithRewriterNumeral(t, m, "3", false)
	r6 := arithRewriterNumeral(t, m, "6", false)
	rNeg2 := arithRewriterNumeral(t, m, "-2", false)
	rTwoThirds := arithRewriterNumeral(t, m, "2/3", false)
	rOneThird := arithRewriterNumeral(t, m, "1/3", false)
	inner, err := r.app(arithOpPower, x, r3)
	astTestMust(t, err)
	astTestPin(t, m, inner)

	status, result := arithRewriterCore(t, r, arithOpPower, inner, r2)
	if status != Rewrite1 {
		t.Fatal("nested power status", status)
	}
	arithRewriterWantOperation(t, r, result, arithOpPower, x, r6)

	status, result = arithRewriterCore(t, r, arithOpPower, x, rNeg2)
	if status != Rewrite2 {
		t.Fatal("negative power status", status)
	}
	_, powerArgs, matched, err := r.operation(result, arithOpPower)
	astTestMust(t, err)
	if !matched || len(powerArgs) != 2 || powerArgs[1] != r2 {
		t.Fatal("negative power shape", matched, len(powerArgs))
	}
	_, divisionArgs, matched, err := r.operation(powerArgs[0], arithOpDiv)
	astTestMust(t, err)
	if !matched || len(divisionArgs) != 2 || divisionArgs[1] != x {
		t.Fatal("negative power inverse", matched, len(divisionArgs))
	}
	arithRewriterWantNumeral(t, m, divisionArgs[0], "1", false)

	status, result = arithRewriterCore(t, r, arithOpPower, x, rTwoThirds)
	if status != Rewrite2 {
		t.Fatal("fractional power status", status)
	}
	_, powerArgs, matched, err = r.operation(result, arithOpPower)
	astTestMust(t, err)
	if !matched || len(powerArgs) != 2 || powerArgs[1] != r2 {
		t.Fatal("fractional outer power", matched, len(powerArgs))
	}
	arithRewriterWantOperation(t, r, powerArgs[0], arithOpPower, x, rOneThird)

	key := "expand_power"
	astTestMust(t, c.SetBoolName(local, &key, true))
	expanding, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, expanding.Close()) })
	status, result = arithRewriterCore(t, expanding, arithOpPower, x, r3)
	if status != Rewrite1 {
		t.Fatal("power expansion status", status)
	}
	arithRewriterWantOperation(t, expanding, result, arithOpMul, x, x, x)
}

// AUDIT.md ast_rewriter_01-F1: mk_power_core's base-1 rule takes any
// exponent, and the zero-base rule any numeral exponent.
func TestArithRewriterPowerUnitAndZeroBase(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	realSort, err := m.MkFamilySort(r.family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "arith-power-unit-x"), realSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	status, result := arithRewriterCore(t, r, arithOpPower, arithRewriterNumeral(t, m, "1", false), x)
	if status != RewriteDone {
		t.Fatal("unit base status", status)
	}
	arithRewriterWantNumeral(t, m, result, "1", false)
	status, result = arithRewriterCore(t, r, arithOpPower, arithRewriterNumeral(t, m, "0", false), arithRewriterNumeral(t, m, "-1/2", false))
	if status != RewriteDone {
		t.Fatal("zero base, fractional exponent status", status)
	}
	arithRewriterWantNumeral(t, m, result, "0", false)
	status, result = arithRewriterCore(t, r, arithOpPower, arithRewriterNumeral(t, m, "0", true), arithRewriterNumeral(t, m, "100000000000000000000", true))
	if status != RewriteDone {
		t.Fatal("zero base, huge exponent status", status)
	}
	arithRewriterWantNumeral(t, m, result, "0", true)
}

func TestArithRewriterComparisonPowerReductionAndToRealElimination(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	iSort, err := m.MkFamilySort(r.family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "arith-compare-power-x"), iSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	y, err := m.MkConst(astTestSymbol(t, m, "arith-compare-power-y"), iSort)
	astTestMust(t, err)
	astTestPin(t, m, y)
	i0 := arithRewriterNumeral(t, m, "0", true)
	i2 := arithRewriterNumeral(t, m, "2", true)
	iNeg2 := arithRewriterNumeral(t, m, "-2", true)
	i4 := arithRewriterNumeral(t, m, "4", true)
	power4, err := r.app(arithOpPower, x, i4)
	astTestMust(t, err)
	astTestPin(t, m, power4)

	status, result := arithRewriterCore(t, r, arithOpLE, power4, i0)
	if status != Rewrite1 {
		t.Fatal("comparison power reduction status", status)
	}
	_, comparisonArgs, matched, err := r.operation(result, arithOpLE)
	astTestMust(t, err)
	if !matched || len(comparisonArgs) != 2 || comparisonArgs[1] != i0 {
		t.Fatal("comparison power reduction", matched, len(comparisonArgs))
	}
	arithRewriterWantOperation(t, r, comparisonArgs[0], arithOpPower, x, i2)

	equalityOut := astTestPin(t, m, power4)
	status, err = r.MkEqCore(power4, i0, equalityOut)
	astTestMust(t, err)
	result, err = m.RefGet(equalityOut)
	astTestMust(t, err)
	if status != Rewrite1 {
		t.Fatal("equality power reduction status", status)
	}
	kind, err := m.GetDeclKind(result)
	astTestMust(t, err)
	eqLeft, err := m.GetArg(result, 0)
	astTestMust(t, err)
	eqRight, err := m.GetArg(result, 1)
	astTestMust(t, err)
	if kind != ast.ASTOpEq || eqLeft != x || eqRight != i0 {
		t.Fatal("equality power reduction", kind, eqLeft, eqRight)
	}

	key := "elim_to_real"
	astTestMust(t, c.SetBoolName(local, &key, true))
	eliminating, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, eliminating.Close()) })
	toRealX, err := eliminating.app(arithOpToReal, x)
	astTestMust(t, err)
	astTestPin(t, m, toRealX)
	toRealY, err := eliminating.app(arithOpToReal, y)
	astTestMust(t, err)
	astTestPin(t, m, toRealY)
	r2 := arithRewriterNumeral(t, m, "2", false)
	realLeft, err := eliminating.app(arithOpAdd, toRealX, r2)
	astTestMust(t, err)
	astTestPin(t, m, realLeft)
	status, result = arithRewriterCore(t, eliminating, arithOpLE, realLeft, toRealY)
	if status != RewriteDone {
		t.Fatal("to-real comparison elimination status", status)
	}
	_, comparisonArgs, matched, err = eliminating.operation(result, arithOpLE)
	astTestMust(t, err)
	if !matched || len(comparisonArgs) != 2 || comparisonArgs[0] != x {
		t.Fatal("to-real comparison elimination", matched, len(comparisonArgs))
	}
	arithRewriterWantOperation(t, eliminating, comparisonArgs[1], arithOpAdd, iNeg2, y)
}

func TestArithRewriterPolynomialGCDReduction(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	key := "gcd_rounding"
	astTestMust(t, c.SetBoolName(local, &key, true))
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	iSort, err := m.MkFamilySort(r.family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "arith-gcd-x"), iSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	y, err := m.MkConst(astTestSymbol(t, m, "arith-gcd-y"), iSort)
	astTestMust(t, err)
	astTestPin(t, m, y)
	i2 := arithRewriterNumeral(t, m, "2", true)
	i4 := arithRewriterNumeral(t, m, "4", true)
	i5 := arithRewriterNumeral(t, m, "5", true)
	twoX, err := r.app(arithOpMul, i2, x)
	astTestMust(t, err)
	astTestPin(t, m, twoX)
	fourY, err := r.app(arithOpMul, i4, y)
	astTestMust(t, err)
	astTestPin(t, m, fourY)
	polynomial, err := r.app(arithOpAdd, twoX, fourY)
	astTestMust(t, err)
	astTestPin(t, m, polynomial)

	status, result := arithRewriterCore(t, r, arithOpLE, polynomial, i5)
	if status != RewriteDone {
		t.Fatal("GCD inequality reduction status", status)
	}
	_, comparisonArgs, matched, err := r.operation(result, arithOpLE)
	astTestMust(t, err)
	if !matched || len(comparisonArgs) != 2 {
		t.Fatal("GCD inequality comparison", matched, len(comparisonArgs))
	}
	arithRewriterWantNumeral(t, m, comparisonArgs[1], "2", true)
	_, lhsArgs, matched, err := r.operation(comparisonArgs[0], arithOpAdd)
	astTestMust(t, err)
	if !matched || len(lhsArgs) != 2 || lhsArgs[0] != x {
		t.Fatal("GCD quotient polynomial", matched, len(lhsArgs))
	}
	arithRewriterWantOperation(t, r, lhsArgs[1], arithOpMul, i2, y)

	equalityOut := astTestPin(t, m, polynomial)
	status, err = r.MkEqCore(polynomial, i5, equalityOut)
	astTestMust(t, err)
	result, err = m.RefGet(equalityOut)
	astTestMust(t, err)
	if status != RewriteDone || !m.IsFalse(result) {
		t.Fatal("GCD equality divisibility", status, result)
	}
}

func TestArithRewriterAlgebraicComparisonsAndArithmetic(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	previous, err := uparams.BindProcessGlobalParamsService(g)
	astTestMust(t, err)
	t.Cleanup(func() {
		_, e := uparams.BindProcessGlobalParamsService(previous)
		astTestMust(t, e)
	})
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	manager, err := m.ArithmeticAlgebraicManager()
	astTestMust(t, err)
	makeRoot := func(n int32) ast.ASTHandle {
		integer := unumeral.NewInt(n)
		var basic, root polynomial.Algebraic
		astTestMust(t, manager.SetInteger(&basic, &integer))
		astTestMust(t, manager.Root(&basic, 2, &root))
		astTestMust(t, manager.Del(&basic))
		h, e := m.ArithmeticAlgebraicNumeral(&root)
		astTestMust(t, e)
		astTestPin(t, m, h)
		astTestMust(t, manager.Del(&root))
		return h
	}
	sqrt2, sqrt3 := makeRoot(2), makeRoot(3)

	status, result := arithRewriterCore(t, r, arithOpLE, sqrt2, sqrt3)
	if status != RewriteDone || !m.IsTrue(result) {
		t.Fatal("algebraic comparison", status, result)
	}
	equalityOut := astTestPin(t, m, sqrt2)
	status, err = r.MkEqCore(sqrt2, sqrt3, equalityOut)
	astTestMust(t, err)
	result, err = m.RefGet(equalityOut)
	astTestMust(t, err)
	if status != RewriteDone || !m.IsFalse(result) {
		t.Fatal("algebraic equality", status, result)
	}

	status, result = arithRewriterCore(t, r, arithOpAdd, sqrt2, sqrt2)
	if status != RewriteDone {
		t.Fatal("algebraic addition status", status)
	}
	var sum polynomial.Algebraic
	matched, err := m.ArithmeticAlgebraicNumeralValue(result, &sum)
	astTestMust(t, err)
	if !matched {
		t.Fatal("algebraic addition result")
	}
	degree, err := manager.Degree(&sum)
	astTestMust(t, err)
	if degree != 2 {
		t.Fatal("algebraic addition degree", degree)
	}
	astTestMust(t, manager.Del(&sum))

	status, result = arithRewriterCore(t, r, arithOpMul, sqrt2, sqrt2)
	if status != RewriteDone {
		t.Fatal("algebraic multiplication status", status)
	}
	arithRewriterWantNumeral(t, m, result, "2", false)
	status, result = arithRewriterCore(t, r, arithOpDiv, sqrt2, sqrt2)
	if status != RewriteDone {
		t.Fatal("algebraic division status", status)
	}
	arithRewriterWantNumeral(t, m, result, "1", false)

	r2 := arithRewriterNumeral(t, m, "2", false)
	rHalf := arithRewriterNumeral(t, m, "1/2", false)
	status, result = arithRewriterCore(t, r, arithOpPower, sqrt2, r2)
	if status != RewriteDone {
		t.Fatal("algebraic square status", status)
	}
	arithRewriterWantNumeral(t, m, result, "2", false)
	status, result = arithRewriterCore(t, r, arithOpPower, r2, rHalf)
	if status != RewriteDone {
		t.Fatal("algebraic root status", status)
	}
	var root polynomial.Algebraic
	matched, err = m.ArithmeticAlgebraicNumeralValue(result, &root)
	astTestMust(t, err)
	if !matched {
		t.Fatal("algebraic root result")
	}
	var expected polynomial.Algebraic
	matched, err = m.ArithmeticAlgebraicNumeralValue(sqrt2, &expected)
	astTestMust(t, err)
	if !matched {
		t.Fatal("algebraic expected root")
	}
	comparison, err := manager.Compare(&root, &expected)
	astTestMust(t, err)
	if comparison != 0 {
		t.Fatal("algebraic root value", comparison)
	}
	astTestMust(t, manager.Del(&root))
	astTestMust(t, manager.Del(&expected))
}

func TestArithRewriterTranscendentalCore(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	realSort, err := m.MkFamilySort(r.family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "arith-trans-x"), realSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	r0 := arithRewriterNumeral(t, m, "0", false)
	r1 := arithRewriterNumeral(t, m, "1", false)
	rNeg1 := arithRewriterNumeral(t, m, "-1", false)
	rHalf := arithRewriterNumeral(t, m, "1/2", false)
	pi, err := r.pi()
	astTestMust(t, err)
	astTestPin(t, m, pi)
	halfPi, err := r.app(arithOpMul, rHalf, pi)
	astTestMust(t, err)
	astTestPin(t, m, halfPi)

	for _, row := range []struct {
		kind int32
		arg  ast.ASTHandle
		want string
	}{
		{arithOpSin, r0, "0"},
		{arithOpCos, r0, "1"},
		{arithOpSin, halfPi, "1"},
		{arithOpCos, pi, "-1"},
		{arithOpTan, pi, "0"},
	} {
		status, result := arithRewriterCore(t, r, row.kind, row.arg)
		if status == RewriteFailed {
			t.Fatal("transcendental numeral status", row.kind, status)
		}
		arithRewriterWantNumeral(t, m, result, row.want, false)
	}

	asinX, err := r.app(arithOpASin, x)
	astTestMust(t, err)
	astTestPin(t, m, asinX)
	status, result := arithRewriterCore(t, r, arithOpSin, asinX)
	if status != RewriteDone || result != x {
		t.Fatal("sin/asin cancellation", status, result, x)
	}
	status, result = arithRewriterCore(t, r, arithOpATan, r1)
	if status != Rewrite2 {
		t.Fatal("atan one status", status)
	}
	arithRewriterWantOperation(t, r, result, arithOpMul, arithRewriterNumeral(t, m, "1/4", false), pi)

	asinhX, err := r.app(arithOpASinh, x)
	astTestMust(t, err)
	astTestPin(t, m, asinhX)
	status, result = arithRewriterCore(t, r, arithOpSinh, asinhX)
	if status != RewriteDone || result != x {
		t.Fatal("sinh/asinh cancellation", status, result, x)
	}
	negativeX, err := r.app(arithOpMul, rNeg1, x)
	astTestMust(t, err)
	astTestPin(t, m, negativeX)
	status, result = arithRewriterCore(t, r, arithOpCosh, negativeX)
	if status != RewriteDone {
		t.Fatal("cosh evenness status", status)
	}
	arithRewriterWantOperation(t, r, result, arithOpCosh, x)

	key := "expand_tan"
	astTestMust(t, c.SetBoolName(local, &key, true))
	expanding, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, expanding.Close()) })
	status, result = arithRewriterCore(t, expanding, arithOpTan, x)
	if status != Rewrite2 {
		t.Fatal("tan expansion status", status)
	}
	_, divisionArgs, matched, err := expanding.operation(result, arithOpDiv)
	astTestMust(t, err)
	if !matched || len(divisionArgs) != 2 {
		t.Fatal("tan expansion", matched, len(divisionArgs))
	}
	arithRewriterWantOperation(t, expanding, divisionArgs[0], arithOpSin, x)
	arithRewriterWantOperation(t, expanding, divisionArgs[1], arithOpCos, x)

	offset, err := r.app(arithOpAdd, x, halfPi)
	astTestMust(t, err)
	astTestPin(t, m, offset)
	status, result = arithRewriterCore(t, r, arithOpSin, offset)
	if status != Rewrite2 {
		t.Fatal("sine pi offset status", status)
	}
	_, cosineArgs, matched, err := r.operation(result, arithOpCos)
	astTestMust(t, err)
	if !matched || len(cosineArgs) != 1 {
		t.Fatal("sine pi offset", matched, len(cosineArgs))
	}
	arithRewriterWantOperation(t, r, cosineArgs[0], arithOpSub, offset, halfPi)

	offset, err = r.app(arithOpAdd, x, pi)
	astTestMust(t, err)
	astTestPin(t, m, offset)
	status, result = arithRewriterCore(t, r, arithOpCos, offset)
	if status != Rewrite3 {
		t.Fatal("cosine pi offset status", status)
	}
	_, negativeArgs, matched, err := r.operation(result, arithOpUMinus)
	astTestMust(t, err)
	if !matched || len(negativeArgs) != 1 {
		t.Fatal("cosine pi offset negative", matched, len(negativeArgs))
	}
	_, cosineArgs, matched, err = r.operation(negativeArgs[0], arithOpCos)
	astTestMust(t, err)
	if !matched || len(cosineArgs) != 1 {
		t.Fatal("cosine pi offset cosine", matched, len(cosineArgs))
	}
	arithRewriterWantOperation(t, r, cosineArgs[0], arithOpSub, offset, pi)

	status, result = arithRewriterCore(t, r, arithOpTan, offset)
	if status != Rewrite2 {
		t.Fatal("tangent pi offset status", status)
	}
	_, tangentArgs, matched, err := r.operation(result, arithOpTan)
	astTestMust(t, err)
	if !matched || len(tangentArgs) != 1 {
		t.Fatal("tangent pi offset", matched, len(tangentArgs))
	}
	arithRewriterWantOperation(t, r, tangentArgs[0], arithOpSub, offset, pi)

	intSort, err := m.MkFamilySort(r.family, 1, nil)
	astTestMust(t, err)
	n, err := m.MkConst(astTestSymbol(t, m, "arith-trans-n"), intSort)
	astTestMust(t, err)
	astTestPin(t, m, n)
	toRealN, err := r.app(arithOpToReal, n)
	astTestMust(t, err)
	piN, err := r.app(arithOpMul, pi, toRealN)
	astTestMust(t, err)
	r2 := arithRewriterNumeral(t, m, "2", false)
	twoPiN, err := r.app(arithOpMul, r2, piN)
	astTestMust(t, err)
	dynamicOffset, err := r.app(arithOpAdd, x, twoPiN)
	astTestMust(t, err)
	status, result = arithRewriterCore(t, r, arithOpSin, dynamicOffset)
	if status != Rewrite2 {
		t.Fatal("dynamic sine period status", status)
	}
	_, sineArgs, matched, err := r.operation(result, arithOpSin)
	astTestMust(t, err)
	if !matched || len(sineArgs) != 1 {
		t.Fatal("dynamic sine period", matched, len(sineArgs))
	}
	arithRewriterWantOperation(t, r, sineArgs[0], arithOpSub, dynamicOffset, twoPiN)

	twelfth := arithRewriterNumeral(t, m, "1/12", false)
	twelfthPi, err := r.app(arithOpMul, twelfth, pi)
	astTestMust(t, err)
	status, result = arithRewriterCore(t, r, arithOpSin, twelfthPi)
	if status != RewriteFull {
		t.Fatal("twelfth-angle sine status", status)
	}
	if _, _, matched, err = r.operation(result, arithOpDiv); err != nil || !matched {
		t.Fatal("twelfth-angle sine division", matched, err)
	}
}

func TestArithRewriterAdvancedPolynomialModes(t *testing.T) {
	newFixture := func(t *testing.T, settings map[string]bool, blowup *uint32) (*ArithRewriter, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
		t.Helper()
		m, c, g, local := boolRewriterConfigFixture(t)
		for key, value := range settings {
			name := key
			astTestMust(t, c.SetBoolName(local, &name, value))
		}
		if blowup != nil {
			name := "som_blowup"
			astTestMust(t, c.SetUIntName(local, &name, *blowup))
		}
		r, err := NewArithRewriter(m, c, g, local)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		iSort, err := m.MkFamilySort(r.family, 1, nil)
		astTestMust(t, err)
		variable := func(name string) ast.ASTHandle {
			h, err := m.MkConst(astTestSymbol(t, m, name), iSort)
			astTestMust(t, err)
			astTestPin(t, m, h)
			return h
		}
		return r, variable("advanced-poly-x"), variable("advanced-poly-y"), variable("advanced-poly-z")
	}

	t.Run("sum-of-monomials", func(t *testing.T) {
		r, x, y, z := newFixture(t, map[string]bool{"som": true}, nil)
		left, err := r.app(arithOpAdd, x, y)
		astTestMust(t, err)
		astTestPin(t, r.manager, left)
		right, err := r.app(arithOpAdd, y, z)
		astTestMust(t, err)
		astTestPin(t, r.manager, right)
		status, result := arithRewriterCore(t, r, arithOpMul, left, right)
		if status != Rewrite2 {
			t.Fatal("sum-of-monomials status", status)
		}
		_, monomials, matched, err := r.operation(result, arithOpAdd)
		astTestMust(t, err)
		if !matched || len(monomials) != 4 {
			t.Fatal("sum-of-monomials expansion", matched, len(monomials))
		}
		for _, monomial := range monomials {
			_, factors, product, err := r.operation(monomial, arithOpMul)
			astTestMust(t, err)
			if !product || len(factors) != 2 {
				t.Fatal("sum-of-monomials product", product, len(factors))
			}
		}
	})

	t.Run("sum-of-monomials-blowup", func(t *testing.T) {
		limit := uint32(0)
		r, x, y, z := newFixture(t, map[string]bool{"som": true}, &limit)
		left, err := r.app(arithOpAdd, x, y)
		astTestMust(t, err)
		right, err := r.app(arithOpAdd, y, z)
		astTestMust(t, err)
		app, err := r.manager.MkFamilyApp(r.family, arithOpMul, nil, []ast.ASTHandle{left, right})
		astTestMust(t, err)
		decl, err := r.manager.GetDecl(app)
		astTestMust(t, err)
		out := astTestPin(t, r.manager, app)
		status, err := r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{left, right}), out)
		if status != RewriteFailed || !errors.Is(err, ErrArithPolynomialMonomialBlowup) {
			t.Fatal("sum-of-monomials blowup", status, err)
		}
	})

	for _, flat := range []bool{false, true} {
		t.Run(fmt.Sprintf("coefficient-distribution-flat-%t", flat), func(t *testing.T) {
			r, x, y, _ := newFixture(t, map[string]bool{"flat": flat}, nil)
			i2 := arithRewriterNumeral(t, r.manager, "2", true)
			sum, err := r.app(arithOpAdd, x, y)
			astTestMust(t, err)
			astTestPin(t, r.manager, sum)
			status, result := arithRewriterCore(t, r, arithOpMul, i2, sum)
			if status != Rewrite2 {
				t.Fatal("coefficient distribution status", status)
			}
			_, terms, matched, err := r.operation(result, arithOpAdd)
			astTestMust(t, err)
			if !matched || len(terms) != 2 {
				t.Fatal("coefficient distribution", matched, len(terms))
			}
			arithRewriterWantOperation(t, r, terms[0], arithOpMul, i2, x)
			arithRewriterWantOperation(t, r, terms[1], arithOpMul, i2, y)
		})
	}

	t.Run("coefficient-hoisting", func(t *testing.T) {
		r, x, y, _ := newFixture(t, map[string]bool{"hoist_cmul": true}, nil)
		i2 := arithRewriterNumeral(t, r.manager, "2", true)
		sum, err := r.app(arithOpAdd, x, y)
		astTestMust(t, err)
		astTestPin(t, r.manager, sum)
		status, result := arithRewriterCore(t, r, arithOpMul, i2, sum)
		if status != RewriteFailed {
			t.Fatal("hoisted coefficient must remain factored", status, result)
		}

		left, err := r.app(arithOpMul, i2, x)
		astTestMust(t, err)
		right, err := r.app(arithOpMul, i2, y)
		astTestMust(t, err)
		status, result = arithRewriterCore(t, r, arithOpAdd, left, right)
		if status != RewriteDone {
			t.Fatal("coefficient hoisting status", status)
		}
		_, factors, matched, err := r.operation(result, arithOpMul)
		astTestMust(t, err)
		if !matched || len(factors) != 2 || factors[0] != i2 {
			t.Fatal("coefficient hoisting", matched, len(factors))
		}
		arithRewriterWantOperation(t, r, factors[1], arithOpAdd, x, y)
	})
	t.Run("single-factor-before-sum-of-monomials", func(t *testing.T) {
		r, x, y, _ := newFixture(t, map[string]bool{"som": true, "hoist_cmul": true}, nil)
		sum, err := r.app(arithOpAdd, x, y)
		astTestMust(t, err)
		astTestPin(t, r.manager, sum)
		i2 := arithRewriterNumeral(t, r.manager, "2", true)
		status, _ := arithRewriterCore(t, r, arithOpMul, i2, sum)
		if status != RewriteFailed {
			t.Fatal("SOM expanded a hoisted single factor", status)
		}
		i1 := arithRewriterNumeral(t, r.manager, "1", true)
		status, result := arithRewriterCore(t, r, arithOpMul, i1, sum)
		if status != RewriteDone || result != sum {
			t.Fatal("unit coefficient must return the single factor", status, result, sum)
		}
	})

	t.Run("pairwise-multiplication-hoisting", func(t *testing.T) {
		r, x, y, z := newFixture(t, map[string]bool{"hoist_mul": true}, nil)
		left, err := r.app(arithOpMul, x, y)
		astTestMust(t, err)
		right, err := r.app(arithOpMul, x, z)
		astTestMust(t, err)
		status, result := arithRewriterCore(t, r, arithOpAdd, left, right, y)
		if status != RewriteFull {
			t.Fatal("multiplication hoisting status", status)
		}
		_, terms, matched, err := r.operation(result, arithOpAdd)
		astTestMust(t, err)
		if !matched || len(terms) != 3 {
			t.Fatal("pairwise multiplication hoisting", matched, len(terms))
		}
		_, factors, matched, err := r.operation(terms[0], arithOpMul)
		astTestMust(t, err)
		if !matched || len(factors) != 2 || factors[0] != x {
			t.Fatal("hoisted multiplication", matched, len(factors))
		}
		arithRewriterWantOperation(t, r, factors[1], arithOpAdd, y, z)
		arithRewriterWantNumeral(t, r.manager, terms[1], "0", true)
		if terms[2] != y {
			t.Fatal("unrelated summand changed", terms[2], y)
		}
	})

	t.Run("mode-normalization", func(t *testing.T) {
		r, _, _, _ := newFixture(t, map[string]bool{"som": true, "hoist_mul": true}, nil)
		if !r.sumOfMonomials || r.hoistMul {
			t.Fatal("sum-of-monomials must disable multiplication hoisting", r.sumOfMonomials, r.hoistMul)
		}
		r, _, _, _ = newFixture(t, map[string]bool{"flat": false, "som": true}, nil)
		if r.sumOfMonomials {
			t.Fatal("non-flat mode retained sum-of-monomials")
		}
	})
}

func TestArithRewriterRemainingStructuralNormalization(t *testing.T) {
	t.Run("coefficient-product-flattening-guard", func(t *testing.T) {
		m, c, g, local := boolRewriterConfigFixture(t)
		r, err := NewArithRewriter(m, c, g, local)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		iSort, err := m.MkFamilySort(r.family, 1, nil)
		astTestMust(t, err)
		x, err := m.MkConst(astTestSymbol(t, m, "arith-flat-guard-x"), iSort)
		astTestMust(t, err)
		astTestPin(t, m, x)
		y, err := m.MkConst(astTestSymbol(t, m, "arith-flat-guard-y"), iSort)
		astTestMust(t, err)
		astTestPin(t, m, y)
		two := arithRewriterNumeral(t, m, "2", true)
		product, err := r.app(arithOpMul, x, y)
		astTestMust(t, err)
		status, _ := arithRewriterCore(t, r, arithOpMul, two, product)
		if status != RewriteFailed {
			t.Fatal("coefficient times plain product must retain source normal form", status)
		}
	})

	t.Run("inequality-order-and-equality-movement", func(t *testing.T) {
		m, c, g, local := boolRewriterConfigFixture(t)
		key := "arith_ineq_lhs"
		astTestMust(t, c.SetBoolName(local, &key, true))
		r, err := NewArithRewriter(m, c, g, local)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		iSort, err := m.MkFamilySort(r.family, 1, nil)
		astTestMust(t, err)
		x, err := m.MkConst(astTestSymbol(t, m, "arith-ordinal-x"), iSort)
		astTestMust(t, err)
		astTestPin(t, m, x)
		y, err := m.MkConst(astTestSymbol(t, m, "arith-ordinal-y"), iSort)
		astTestMust(t, err)
		astTestPin(t, m, y)
		xID, err := m.GetASTID(x)
		astTestMust(t, err)
		yID, err := m.GetASTID(y)
		astTestMust(t, err)
		less, err := r.monomialLess(x, y)
		astTestMust(t, err)
		if less != (xID < yID) {
			t.Fatal("inequality monomial ordinal", less, xID, yID)
		}
		arithmeticLeft, err := r.app(arithOpAdd, x, x)
		astTestMust(t, err)
		out := astTestPin(t, m, x)
		status, err := r.MkEqCore(arithmeticLeft, y, out)
		astTestMust(t, err)
		if status != RewriteDone {
			t.Fatal("inequality-mode equality movement status", status)
		}
		result, err := m.RefGet(out)
		astTestMust(t, err)
		kind, err := m.GetDeclKind(result)
		astTestMust(t, err)
		right, err := m.GetArg(result, 1)
		astTestMust(t, err)
		arithRewriterWantNumeral(t, m, right, "0", true)
		if kind != ast.ASTOpEq {
			t.Fatal("inequality-mode equality movement result", kind)
		}
	})

	t.Run("exact-root-without-algebraic-evaluator", func(t *testing.T) {
		m, c, g, local := boolRewriterConfigFixture(t)
		key := "algebraic_number_evaluator"
		astTestMust(t, c.SetBoolName(local, &key, false))
		r, err := NewArithRewriter(m, c, g, local)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		four := arithRewriterNumeral(t, m, "4", false)
		half := arithRewriterNumeral(t, m, "1/2", false)
		status, result := arithRewriterCore(t, r, arithOpPower, four, half)
		if status != RewriteDone {
			t.Fatal("exact root status", status)
		}
		arithRewriterWantNumeral(t, m, result, "2", false)
	})

	t.Run("sum-of-monomials-power-expansion", func(t *testing.T) {
		m, c, g, local := boolRewriterConfigFixture(t)
		key := "som"
		astTestMust(t, c.SetBoolName(local, &key, true))
		r, err := NewArithRewriter(m, c, g, local)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		iSort, err := m.MkFamilySort(r.family, 1, nil)
		astTestMust(t, err)
		x, err := m.MkConst(astTestSymbol(t, m, "arith-som-power-x"), iSort)
		astTestMust(t, err)
		astTestPin(t, m, x)
		y, err := m.MkConst(astTestSymbol(t, m, "arith-som-power-y"), iSort)
		astTestMust(t, err)
		astTestPin(t, m, y)
		sum, err := r.app(arithOpAdd, x, y)
		astTestMust(t, err)
		two := arithRewriterNumeral(t, m, "2", true)
		status, result := arithRewriterCore(t, r, arithOpPower, sum, two)
		if status != Rewrite1 {
			t.Fatal("sum-of-monomials power status", status)
		}
		arithRewriterWantOperation(t, r, result, arithOpMul, sum, sum)
	})

	t.Run("division-coefficients", func(t *testing.T) {
		m, c, g, local := boolRewriterConfigFixture(t)
		r, err := NewArithRewriter(m, c, g, local)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		realSort, err := m.MkFamilySort(r.family, 0, nil)
		astTestMust(t, err)
		x, err := m.MkConst(astTestSymbol(t, m, "arith-div-coefficient-x"), realSort)
		astTestMust(t, err)
		astTestPin(t, m, x)
		y, err := m.MkConst(astTestSymbol(t, m, "arith-div-coefficient-y"), realSort)
		astTestMust(t, err)
		astTestPin(t, m, y)
		r2 := arithRewriterNumeral(t, m, "2", false)
		r3 := arithRewriterNumeral(t, m, "3", false)
		left, err := r.app(arithOpMul, r2, x)
		astTestMust(t, err)
		right, err := r.app(arithOpMul, r3, y)
		astTestMust(t, err)
		status, result := arithRewriterCore(t, r, arithOpDiv, left, right)
		if status != Rewrite2 {
			t.Fatal("division coefficient status", status)
		}
		_, factors, matched, err := r.operation(result, arithOpMul)
		astTestMust(t, err)
		if !matched || len(factors) != 2 {
			t.Fatal("division coefficient product", matched, len(factors))
		}
		arithRewriterWantNumeral(t, m, factors[0], "2/3", false)
		arithRewriterWantOperation(t, r, factors[1], arithOpDiv, x, y)
	})

	t.Run("negative-polynomial-orientation", func(t *testing.T) {
		m, c, g, local := boolRewriterConfigFixture(t)
		key := "arith_lhs"
		astTestMust(t, c.SetBoolName(local, &key, true))
		r, err := NewArithRewriter(m, c, g, local)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		iSort, err := m.MkFamilySort(r.family, 1, nil)
		astTestMust(t, err)
		x, err := m.MkConst(astTestSymbol(t, m, "arith-negative-poly-x"), iSort)
		astTestMust(t, err)
		astTestPin(t, m, x)
		y, err := m.MkConst(astTestSymbol(t, m, "arith-negative-poly-y"), iSort)
		astTestMust(t, err)
		astTestPin(t, m, y)
		minusTwo := arithRewriterNumeral(t, m, "-2", true)
		minusThree := arithRewriterNumeral(t, m, "-3", true)
		five := arithRewriterNumeral(t, m, "5", true)
		leftTerm, err := r.app(arithOpMul, minusTwo, x)
		astTestMust(t, err)
		rightTerm, err := r.app(arithOpMul, minusThree, y)
		astTestMust(t, err)
		polynomial, err := r.app(arithOpAdd, leftTerm, rightTerm)
		astTestMust(t, err)
		status, result := arithRewriterCore(t, r, arithOpLE, polynomial, five)
		if status != RewriteDone {
			t.Fatal("negative polynomial status", status)
		}
		_, comparison, matched, err := r.operation(result, arithOpGE)
		astTestMust(t, err)
		if !matched || len(comparison) != 2 {
			t.Fatal("negative polynomial comparison orientation", matched, len(comparison))
		}
		_, terms, matched, err := r.operation(comparison[0], arithOpAdd)
		astTestMust(t, err)
		if !matched || len(terms) == 0 {
			t.Fatal("negative polynomial positive left side", matched, len(terms))
		}
		coefficient, _, numeral, err := r.numeral(func() ast.ASTHandle {
			_, factors, product, e := r.operation(terms[0], arithOpMul)
			astTestMust(t, e)
			if !product || len(factors) == 0 {
				t.Fatal("negative polynomial first monomial", product, len(factors))
			}
			return factors[0]
		}())
		astTestMust(t, err)
		if !numeral || coefficient.Sign() <= 0 {
			t.Fatal("negative polynomial coefficient", coefficient, numeral)
		}
	})
}
