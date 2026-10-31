package arith

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func cardinalityBool(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	sort, err := m.MkBoolSort()
	equationMust(t, err)
	sym, err := m.Symbols.Intern(name)
	equationMust(t, err)
	h, err := m.MkConst(sym, sort)
	equationMust(t, err)
	fixDLPin(t, m, h)
	return h
}

func cardinalityIte(t *testing.T, m *ast.ASTManager, c, a, b ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	h, err := m.MkIte(c, a, b)
	equationMust(t, err)
	fixDLPin(t, m, h)
	return h
}

// Evaluate only the Boolean language emitted by the sum extractor. Expected
// arithmetic values below are calculated independently for every assignment.
func cardinalityCondition(t *testing.T, m *ast.ASTManager, h ast.ASTHandle, bools map[ast.ASTHandle]bool, ints map[ast.ASTHandle]int64) bool {
	t.Helper()
	if m.IsTrue(h) {
		return true
	}
	if m.IsFalse(h) {
		return false
	}
	if b, ok := bools[h]; ok {
		return b
	}
	op, err := m.GetDeclKind(h)
	equationMust(t, err)
	n, err := m.GetNumArgs(h)
	equationMust(t, err)
	if op == ast.ASTOpAnd {
		result := true
		for i := uint32(0); i < n; i++ {
			a, err := m.GetArg(h, i)
			equationMust(t, err)
			result = cardinalityCondition(t, m, a, bools, ints) && result
		}
		return result
	}
	a, err := m.GetArg(h, 0)
	equationMust(t, err)
	if op == ast.ASTOpNot {
		return !cardinalityCondition(t, m, a, bools, ints)
	}
	if op == ast.ASTOpEq {
		b, err := m.GetArg(h, 1)
		equationMust(t, err)
		text, _, found, err := m.ArithmeticNumeralText(b)
		equationMust(t, err)
		value, ok := ints[a]
		if !found || !ok {
			t.Fatal("unexpected indicator", h)
		}
		n, ok := new(big.Rat).SetString(text)
		if !ok {
			t.Fatal(text)
		}
		return n.Cmp(big.NewRat(value, 1)) == 0
	}
	t.Fatal("unexpected Boolean condition", h, op)
	return false
}

func cardinalityValue(t *testing.T, m *ast.ASTManager, s *PBSum, bools map[ast.ASTHandle]bool, ints map[ast.ASTHandle]int64) *big.Rat {
	t.Helper()
	v := new(big.Rat).Set(&s.Constant)
	for i, h := range s.Args {
		if s.Coefficients[i].Sign() <= 0 {
			t.Fatal("nonpositive output weight")
		}
		if cardinalityCondition(t, m, h, bools, ints) {
			v.Add(v, &s.Coefficients[i])
		}
	}
	return v
}

func TestLIA2CardWeightedConditionalSum(t *testing.T) {
	_, m := fixDLFixture(t, false)
	p, q := cardinalityBool(t, m, "p"), cardinalityBool(t, m, "q")
	x, y := fixDLConst(t, m, true, "x"), fixDLConst(t, m, true, "y")
	num := func(n string) ast.ASTHandle { return fixDLNumber(t, m, true, n) }
	// 3*x - (if p then 2*y+1 else if q then -4 else 5).
	left := fixDLApp(t, m, 9, num("3"), x)
	y2 := fixDLApp(t, m, 9, y, num("2"))
	y2plus1 := fixDLApp(t, m, 6, y2, num("1"))
	inner := cardinalityIte(t, m, q, num("-4"), num("5"))
	right := cardinalityIte(t, m, p, y2plus1, inner)
	term := fixDLApp(t, m, 7, left, right)
	s, ok, err := ExtractPBSum(m, term, map[ast.ASTHandle]bool{x: true, y: true})
	equationMust(t, err)
	if !ok {
		t.Fatal("supported sum rejected")
	}
	t.Cleanup(func() { equationMust(t, s.Close()) })
	for bits := 0; bits < 16; bits++ {
		pv, qv := bits&1 != 0, bits&2 != 0
		xv, yv := int64((bits>>2)&1), int64((bits>>3)&1)
		rhs := int64(5)
		if pv {
			rhs = 2*yv + 1
		} else if qv {
			rhs = -4
		}
		want := big.NewRat(3*xv-rhs, 1)
		got := cardinalityValue(t, m, s, map[ast.ASTHandle]bool{p: pv, q: qv}, map[ast.ASTHandle]int64{x: xv, y: yv})
		if got.Cmp(want) != 0 {
			t.Fatalf("assignment %04b: got %s want %s", bits, got, want)
		}
	}
	// A separate result owns its Boolean conditions independently.
	other, ok, err := ExtractPBSum(m, term, map[ast.ASTHandle]bool{x: true, y: true})
	equationMust(t, err)
	if !ok {
		t.Fatal("second extraction rejected")
	}
	equationMust(t, s.Close())
	equationMust(t, s.Close())
	got := cardinalityValue(t, m, other, map[ast.ASTHandle]bool{p: false, q: true}, map[ast.ASTHandle]int64{x: 1, y: 0})
	if got.Cmp(big.NewRat(7, 1)) != 0 {
		t.Fatal(got)
	}
	equationMust(t, other.Close())
}

func TestLIA2CardNumeralWrappersFractionsAndFailure(t *testing.T) {
	_, m := fixDLFixture(t, false)
	p := cardinalityBool(t, m, "p")
	three := fixDLNumber(t, m, true, "3")
	negative := fixDLApp(t, m, 8, three)
	toReal := fixDLApp(t, m, 14, negative)
	wrapped := fixDLApp(t, m, 8, toReal)
	term := fixDLApp(t, m, 9, wrapped, cardinalityIte(t, m, p, fixDLNumber(t, m, false, "1/2"), fixDLNumber(t, m, false, "-2/3")))
	s, ok, err := ExtractPBSum(m, term, nil)
	equationMust(t, err)
	if !ok {
		t.Fatal("wrapped numeral rejected")
	}
	for _, pv := range []bool{false, true} {
		want := big.NewRat(-2, 1)
		if pv {
			want = big.NewRat(3, 2)
		}
		got := cardinalityValue(t, m, s, map[ast.ASTHandle]bool{p: pv}, nil)
		if got.Cmp(want) != 0 {
			t.Fatalf("p=%v got %s want %s", pv, got, want)
		}
	}
	equationMust(t, s.Close())
	unknown := fixDLConst(t, m, true, "unknown")
	bad := cardinalityIte(t, m, p, unknown, three)
	for i := 0; i < 3; i++ {
		s, ok, err = ExtractPBSum(m, bad, nil)
		equationMust(t, err)
		if ok || s != nil {
			t.Fatal("unsupported variable accepted")
		}
	}
	// Failed extraction must leave the input roots usable.
	s, ok, err = ExtractPBSum(m, three, nil)
	equationMust(t, err)
	if !ok || len(s.Args) != 0 || s.Constant.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("constant extraction after failure")
	}
	equationMust(t, s.Close())
}
