package rewriter

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func bigIntTest(text string) *big.Int {
	v, _ := new(big.Int).SetString(text, 10)
	return v
}

func bit2IntArithmeticNumeral(t *testing.T, m *ast.ASTManager, value string) ast.ASTHandle {
	t.Helper()
	s := m.NumeralService()
	n, err := s.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Del(&n)) }()
	astTestMust(t, s.SetString(&n, value))
	h, err := m.ArithmeticNumeral(&n, true)
	astTestMust(t, err)
	astTestPin(t, m, h)
	return h
}

func TestBit2IntExtractionAlignmentAndCacheFoundation(t *testing.T) {
	m, _ := bvAddFixture(t)
	b, err := NewBit2Int(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, b.Close()) })
	if bit2IntNumeralBits(bigIntTest("0")) != 1 || bit2IntNumeralBits(bigIntTest("-1")) != 1 || bit2IntNumeralBits(bigIntTest("8")) != 4 {
		t.Fatal("numeral bit widths")
	}

	three := bvAddNum(t, m, 3, 3)
	bv2int, err := m.MkBitVectorToInt(three)
	astTestMust(t, err)
	astTestPin(t, m, bv2int)
	if width, err := b.getB2ISize(bv2int); err != nil || width != 3 {
		t.Fatal("bv2int size", width, err)
	}
	call := bit2IntCall{owner: b}
	defer func() { astTestMust(t, call.close()) }()
	extracted, matched, err := b.extractBV(&call, bv2int)
	astTestMust(t, err)
	if !matched || extracted.bits != three || extracted.width != 3 || extracted.sign {
		t.Fatal("bv2int extraction", matched, extracted)
	}

	minusFive := bit2IntArithmeticNumeral(t, m, "-5")
	extracted, matched, err = b.extractBV(&call, minusFive)
	astTestMust(t, err)
	if !matched || extracted.width != 3 || !extracted.sign {
		t.Fatal("integer extraction", matched, extracted)
	}
	text, matched, err := m.BitVectorNumeralText(extracted.bits)
	astTestMust(t, err)
	if !matched || text != "3" {
		t.Fatal("negative numeral bit-vector normalization", matched, text)
	}

	fiveWide := bvAddNum(t, m, 7, 5)
	left, right, err := b.alignSizes(&call, three, fiveWide)
	astTestMust(t, err)
	if lw, err := m.BitVectorSize(left); err != nil || lw != 5 {
		t.Fatal("left aligned width", lw, err)
	}
	if rw, err := m.BitVectorSize(right); err != nil || rw != 5 || right != fiveWide {
		t.Fatal("right aligned width", rw, right, err)
	}

	astTestMust(t, b.cacheResult(bv2int, left))
	if size, err := b.cache.Size(); err != nil || size != 1 {
		t.Fatal("cache insertion", size, err)
	}
	// flush_cache is act_cache::cleanup: an entry never found since its
	// insertion is unused and is removed.
	astTestMust(t, b.flushCache())
	if size, err := b.cache.Size(); err != nil || size != 0 {
		t.Fatal("cache flush", size, err)
	}
}

func TestBit2IntArithmeticReductions(t *testing.T) {
	m, _ := bvAddFixture(t)
	b, err := NewBit2Int(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, b.Close()) })
	call := bit2IntCall{owner: b}
	defer func() { astTestMust(t, call.close()) }()
	threeSort, err := m.BitVectorSort(3)
	astTestMust(t, err)
	fourSort, err := m.BitVectorSort(4)
	astTestMust(t, err)
	three, err := m.MkVar(0, threeSort)
	astTestMust(t, err)
	astTestPin(t, m, three)
	five, err := m.MkVar(1, fourSort)
	astTestMust(t, err)
	astTestPin(t, m, five)
	zero := bvAddNum(t, m, 0, 3)
	b3, err := m.MkBitVectorToInt(three)
	astTestMust(t, err)
	astTestPin(t, m, b3)
	b5, err := m.MkBitVectorToInt(five)
	astTestMust(t, err)
	astTestPin(t, m, b5)
	b0, err := m.MkBitVectorToInt(zero)
	astTestMust(t, err)
	astTestPin(t, m, b0)

	if result, ok, err := b.mkAdd(&call, b0, b5); err != nil || !ok || result != b5 {
		t.Fatal("zero addition", result, ok, err)
	}
	sum, ok, err := b.mkAdd(&call, b3, b5)
	astTestMust(t, err)
	if !ok {
		t.Fatal("addition declined")
	}
	if family, err := m.GetFamilyID(sum); err != nil || family != b.arithFamily {
		t.Fatal("addition family", family, err)
	}
	if kind, err := m.GetDeclKind(sum); err != nil || kind != 6 {
		t.Fatal("addition distribution", kind, err)
	}

	for _, row := range []struct {
		mode bit2IntComparison
		kind int32
	}{
		{bit2IntLT, ast.ASTOpNot},
		{bit2IntLE, 22},
		{bit2IntEQ, ast.ASTOpEq},
	} {
		result, ok, err := b.mkComp(&call, row.mode, b3, b5)
		astTestMust(t, err)
		if !ok {
			t.Fatal("comparison declined", row.mode)
		}
		kind, err := m.GetDeclKind(result)
		astTestMust(t, err)
		if kind != row.kind {
			t.Fatal("comparison kind", row.mode, kind, row.kind)
		}
	}

	minusTwo := bit2IntArithmeticNumeral(t, m, "-2")
	product, ok, err := b.mkMul(&call, minusTwo, b3)
	astTestMust(t, err)
	if !ok {
		t.Fatal("multiplication declined")
	}
	if family, err := m.GetFamilyID(product); err != nil || family != b.arithFamily {
		t.Fatal("negative product family", family, err)
	}
	if kind, err := m.GetDeclKind(product); err != nil || kind != 8 {
		t.Fatal("negative product sign", kind, err)
	}
	unsigned, err := m.GetArg(product, 0)
	astTestMust(t, err)
	if family, err := m.GetFamilyID(unsigned); err != nil || family != b.arithFamily {
		t.Fatal("unsigned product family", family, err)
	}
	if kind, err := m.GetDeclKind(unsigned); err != nil || kind != 9 {
		t.Fatal("product distribution", kind, err)
	}
}

func TestBit2IntPolynomialDecomposition(t *testing.T) {
	m, _ := bvAddFixture(t)
	b, err := NewBit2Int(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, b.Close()) })
	sort, err := m.BitVectorSort(3)
	astTestMust(t, err)
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	y, err := m.MkVar(1, sort)
	astTestMust(t, err)
	astTestPin(t, m, y)
	bx, err := m.MkBitVectorToInt(x)
	astTestMust(t, err)
	astTestPin(t, m, bx)
	by, err := m.MkBitVectorToInt(y)
	astTestMust(t, err)
	astTestPin(t, m, by)
	minusOne := bit2IntArithmeticNumeral(t, m, "-1")
	negativeY, err := m.MkFamilyApp(b.arithFamily, 9, nil, []ast.ASTHandle{minusOne, by})
	astTestMust(t, err)
	astTestPin(t, m, negativeY)
	four := bit2IntArithmeticNumeral(t, m, "4")
	minusThree := bit2IntArithmeticNumeral(t, m, "-3")
	sum, err := m.MkFamilyApp(b.arithFamily, 6, nil, []ast.ASTHandle{bx, negativeY, four, minusThree})
	astTestMust(t, err)
	astTestPin(t, m, sum)

	call := bit2IntCall{owner: b}
	defer func() { astTestMust(t, call.close()) }()
	pos, neg, matched, err := b.isBVPoly(&call, sum)
	astTestMust(t, err)
	if !matched {
		t.Fatal("polynomial decomposition declined")
	}
	for name, term := range map[string]ast.ASTHandle{"positive": pos, "negative": neg} {
		_, _, decomposed, err := b.isBVPoly(&call, term)
		astTestMust(t, err)
		if !decomposed {
			t.Fatal(name, "side no longer decomposes")
		}
	}

	two := bit2IntArithmeticNumeral(t, m, "2")
	unsupported, err := m.MkFamilyApp(b.arithFamily, 9, nil, []ast.ASTHandle{two, bx})
	astTestMust(t, err)
	astTestPin(t, m, unsupported)
	_, _, matched, err = b.isBVPoly(&call, unsupported)
	astTestMust(t, err)
	if matched {
		t.Fatal("non-minus-one coefficient accepted")
	}
}

func TestBit2IntCompletePostOrderRewrite(t *testing.T) {
	m, _ := bvAddFixture(t)
	b, err := NewBit2Int(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, b.Close()) })
	sort, err := m.BitVectorSort(3)
	astTestMust(t, err)
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	y, err := m.MkVar(1, sort)
	astTestMust(t, err)
	astTestPin(t, m, y)
	bx, err := m.MkBitVectorToInt(x)
	astTestMust(t, err)
	astTestPin(t, m, bx)
	by, err := m.MkBitVectorToInt(y)
	astTestMust(t, err)
	astTestPin(t, m, by)
	sum, err := m.MkFamilyApp(b.arithFamily, 6, nil, []ast.ASTHandle{bx, by})
	astTestMust(t, err)
	astTestPin(t, m, sum)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	astTestMust(t, b.Rewrite(sum, out))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	if family, err := m.GetFamilyID(got); err != nil || family != b.arithFamily {
		t.Fatal("sum result family", family, err)
	}
	if kind, err := m.GetDeclKind(got); err != nil || kind != 6 {
		t.Fatal("sum result operation", kind, err)
	}

	minusY, err := m.MkFamilyApp(b.arithFamily, 8, nil, []ast.ASTHandle{by})
	astTestMust(t, err)
	astTestPin(t, m, minusY)
	difference, err := m.MkFamilyApp(b.arithFamily, 6, nil, []ast.ASTHandle{bx, minusY})
	astTestMust(t, err)
	astTestPin(t, m, difference)
	zero := bit2IntArithmeticNumeral(t, m, "0")
	equality, err := m.MkEq(difference, zero)
	astTestMust(t, err)
	astTestPin(t, m, equality)
	astTestMust(t, b.Rewrite(equality, out))
	got, err = m.RefGet(out)
	astTestMust(t, err)
	if family, err := m.GetFamilyID(got); err != nil || family != m.BasicFamilyID() {
		t.Fatal("equality family", family, err)
	}
	if kind, err := m.GetDeclKind(got); err != nil || kind != ast.ASTOpEq {
		t.Fatal("equality kind", kind, err)
	}
	left, err := m.GetArg(got, 0)
	astTestMust(t, err)
	if isBV, err := m.IsBitVector(left); err != nil || !isBV {
		t.Fatal("equality BV side", isBV, err)
	}

	three := bit2IntArithmeticNumeral(t, m, "3")
	mod, err := m.MkFamilyApp(b.arithFamily, 13, nil, []ast.ASTHandle{bx, three})
	astTestMust(t, err)
	astTestPin(t, m, mod)
	astTestMust(t, b.Rewrite(mod, out))
	got, err = m.RefGet(out)
	astTestMust(t, err)
	if yes, err := m.IsBitVectorOperation(got, 58); err != nil || !yes {
		t.Fatal("mod result", yes, err)
	}
	modBits, err := m.GetArg(got, 0)
	astTestMust(t, err)
	if isBV, err := m.IsBitVector(modBits); err != nil || !isBV {
		t.Fatal("mod bit-vector body", isBV, err)
	}
	call := bit2IntCall{owner: b}
	general, ok, err := b.rewriteMod(&call, difference, three)
	astTestMust(t, err)
	if !ok {
		t.Fatal("general signed polynomial mod declined")
	}
	if yes, err := m.IsBitVectorOperation(general, 58); err != nil || !yes {
		t.Fatal("general mod result", yes, err)
	}
	astTestMust(t, call.close())
}

func TestBit2IntQuantifierBodyRewrite(t *testing.T) {
	m, _ := bvAddFixture(t)
	b, err := NewBit2Int(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, b.Close()) })
	sort, err := m.BitVectorSort(3)
	astTestMust(t, err)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	bv2int, err := m.MkBitVectorToInt(variable)
	astTestMust(t, err)
	one := bit2IntArithmeticNumeral(t, m, "1")
	sum, err := m.MkFamilyApp(b.arithFamily, 6, nil, []ast.ASTHandle{bv2int, one})
	astTestMust(t, err)
	two := bit2IntArithmeticNumeral(t, m, "2")
	body, err := m.MkEq(sum, two)
	astTestMust(t, err)
	name := astTestSymbol(t, m, "q")
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{name}, Body: body})
	astTestMust(t, err)
	astTestPin(t, m, quantifier)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	astTestMust(t, b.Rewrite(quantifier, out))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	newBody, err := m.GetQuantifierBody(got)
	astTestMust(t, err)
	if newBody == body {
		t.Fatal("quantifier body was not rewritten")
	}
	if count, err := m.GetNumDeclarations(got); err != nil || count != 1 {
		t.Fatal("quantifier declarations", count, err)
	}
}

func TestBit2IntComparisonDispatchAndFallback(t *testing.T) {
	m, _ := bvAddFixture(t)
	b, err := NewBit2Int(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, b.Close()) })
	sort, err := m.BitVectorSort(4)
	astTestMust(t, err)
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	y, err := m.MkVar(1, sort)
	astTestMust(t, err)
	astTestPin(t, m, y)
	bx, err := m.MkBitVectorToInt(x)
	astTestMust(t, err)
	astTestPin(t, m, bx)
	by, err := m.MkBitVectorToInt(y)
	astTestMust(t, err)
	astTestPin(t, m, by)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()

	for _, kind := range []int32{2, 3, 4, 5} {
		comparison, err := m.MkFamilyApp(b.arithFamily, kind, nil, []ast.ASTHandle{bx, by})
		astTestMust(t, err)
		astTestPin(t, m, comparison)
		astTestMust(t, b.Rewrite(comparison, out))
		got, err := m.RefGet(out)
		astTestMust(t, err)
		if family, err := m.GetFamilyID(got); err != nil || family == b.arithFamily {
			t.Fatal("arithmetic comparison survived", kind, family, err)
		}
	}

	inner, err := m.MkFamilyApp(b.arithFamily, 6, nil, []ast.ASTHandle{bx, by})
	astTestMust(t, err)
	astTestPin(t, m, inner)
	negated, err := m.MkFamilyApp(b.arithFamily, 8, nil, []ast.ASTHandle{inner})
	astTestMust(t, err)
	astTestPin(t, m, negated)
	astTestMust(t, b.Rewrite(negated, out))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	if kind, err := m.GetDeclKind(got); err != nil || kind != 8 {
		t.Fatal("fallback application kind", kind, err)
	}
	child, err := m.GetArg(got, 0)
	astTestMust(t, err)
	if child == inner {
		t.Fatal("fallback did not use rewritten child")
	}
	if family, err := m.GetFamilyID(child); err != nil || family != b.arithFamily {
		t.Fatal("rewritten fallback child family", family, err)
	}
	if kind, err := m.GetDeclKind(child); err != nil || kind != 6 {
		t.Fatal("rewritten fallback child kind", kind, err)
	}
}

func TestBit2IntRewriteProofPolicy(t *testing.T) {
	m, _, sort := bvElimFixture(t, true)
	b, err := NewBit2Int(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, b.Close()) })
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	bx, err := m.MkBitVectorToInt(x)
	astTestMust(t, err)
	astTestPin(t, m, bx)
	one := bit2IntArithmeticNumeral(t, m, "1")
	sum, err := m.MkFamilyApp(b.arithFamily, 6, nil, []ast.ASTHandle{bx, one})
	astTestMust(t, err)
	astTestPin(t, m, sum)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	proof, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() {
		astTestMust(t, m.DelRef(&proof))
		astTestMust(t, m.DelRef(&out))
	}()
	astTestMust(t, b.RewriteWithProof(sum, out, proof))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	if got == sum {
		t.Fatal("rewrite did not change arithmetic sum")
	}
	p, err := m.RefGet(proof)
	astTestMust(t, err)
	if p == (ast.ASTHandle{}) {
		t.Fatal("changed rewrite omitted proof")
	}
	astTestMust(t, b.RewriteWithProof(x, out, proof))
	if retained, err := m.RefGet(proof); err != nil || retained != p {
		t.Fatal("unchanged rewrite modified proof output", retained, p, err)
	}
}
