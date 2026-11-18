package bit_blaster

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func logicBits(t *testing.T, e *Engine, value int64, width int) ([]ast.ASTRef, []ast.ASTHandle) {
	t.Helper()
	refs, err := e.NumToBits(bbSmall(value), width)
	paramsMust(t, err)
	return refs, arithmeticHandles(t, e, refs)
}

func TestBitBlasterIndependentLogicCircuits(t *testing.T) {
	m, engine := arithmeticEngine(t)
	const width = 4
	const mask int64 = 15
	out, err := m.NewRef(ast.ASTHandle{})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.DelRef(&out)) }()
	truth, err := m.MkTrue()
	paramsMust(t, err)
	falsehood, err := m.MkFalse()
	paramsMust(t, err)

	for a := int64(0); a <= mask; a++ {
		aRefs, aBits := logicBits(t, engine, a, width)
		left, err := engine.MkRotateLeft(aBits, 1)
		paramsMust(t, err)
		if got, want := arithmeticValue(t, engine, left), ((a<<1)|(a>>(width-1)))&mask; got != want {
			t.Fatalf("rotate-left(%d)=%d, want %d", a, got, want)
		}
		arithmeticRelease(t, m, left)
		right, err := engine.MkRotateRight(aBits, 1)
		paramsMust(t, err)
		if got, want := arithmeticValue(t, engine, right), ((a>>1)|(a<<(width-1)))&mask; got != want {
			t.Fatalf("rotate-right(%d)=%d, want %d", a, got, want)
		}
		arithmeticRelease(t, m, right)

		zeroExtended, err := engine.MkZeroExtend(aBits, 2)
		paramsMust(t, err)
		if got := arithmeticValue(t, engine, zeroExtended); got != a {
			t.Fatalf("zero-extend(%d)=%d", a, got)
		}
		arithmeticRelease(t, m, zeroExtended)
		signExtended, err := engine.MkSignExtend(aBits, 2)
		paramsMust(t, err)
		wantSign := a
		if a&8 != 0 {
			wantSign |= 48
		}
		if got := arithmeticValue(t, engine, signExtended); got != wantSign {
			t.Fatalf("sign-extend(%d)=%d, want %d", a, got, wantSign)
		}
		arithmeticRelease(t, m, signExtended)

		absolute, err := engine.MkAbs(aBits)
		paramsMust(t, err)
		wantAbs := a
		if a&8 != 0 {
			wantAbs = (-a) & mask
		}
		if got := arithmeticValue(t, engine, absolute); got != wantAbs {
			t.Fatalf("abs(%d)=%d, want %d", a, got, wantAbs)
		}
		arithmeticRelease(t, m, absolute)

		for b := int64(0); b <= mask; b++ {
			bRefs, bBits := logicBits(t, engine, b, width)
			cases := []struct {
				name string
				call func([]ast.ASTHandle, []ast.ASTHandle) ([]ast.ASTRef, error)
				want int64
			}{
				{"and", engine.MkAnd, a & b},
				{"or", engine.MkOr, a | b},
				{"xor", engine.MkXor, a ^ b},
				{"xnor", engine.MkXnor, (^(a ^ b)) & mask},
				{"nand", engine.MkNand, (^(a & b)) & mask},
				{"nor", engine.MkNor, (^(a | b)) & mask},
			}
			for _, tc := range cases {
				refs, err := tc.call(aBits, bBits)
				paramsMust(t, err)
				if got := arithmeticValue(t, engine, refs); got != tc.want {
					t.Fatalf("%s(%d,%d)=%d, want %d", tc.name, a, b, got, tc.want)
				}
				arithmeticRelease(t, m, refs)
			}

			paramsMust(t, engine.MkEq(aBits, bBits, out))
			equal, err := m.RefGet(out)
			paramsMust(t, err)
			if got, want := m.IsTrue(equal), a == b; got != want {
				t.Fatalf("eq(%d,%d)=%v, want %v", a, b, got, want)
			}
			mux, err := engine.MkMultiplexer(truth, aBits, bBits)
			paramsMust(t, err)
			if got := arithmeticValue(t, engine, mux); got != a {
				t.Fatalf("true mux(%d,%d)=%d", a, b, got)
			}
			arithmeticRelease(t, m, mux)
			mux, err = engine.MkMultiplexer(falsehood, aBits, bBits)
			paramsMust(t, err)
			if got := arithmeticValue(t, engine, mux); got != b {
				t.Fatalf("false mux(%d,%d)=%d", a, b, got)
			}
			arithmeticRelease(t, m, mux)

			sum, carry, err := engine.MkCarrySaveAdder(aBits, bBits, aBits)
			paramsMust(t, err)
			if got, want := (arithmeticValue(t, engine, sum)+2*arithmeticValue(t, engine, carry))&mask, (a+b+a)&mask; got != want {
				t.Fatalf("carry-save(%d,%d,%d)=%d, want %d", a, b, a, got, want)
			}
			arithmeticRelease(t, m, sum)
			arithmeticRelease(t, m, carry)
			arithmeticRelease(t, m, bRefs)
		}

		paramsMust(t, engine.MkIsEq(aBits, uint64(a), out))
		isEqual, err := m.RefGet(out)
		paramsMust(t, err)
		if !m.IsTrue(isEqual) {
			t.Fatalf("is-eq(%d,%d) is false", a, a)
		}
		eqs, err := engine.MkEqs(aBits)
		paramsMust(t, err)
		for i, ref := range eqs {
			h, err := m.RefGet(ref)
			paramsMust(t, err)
			if got, want := m.IsTrue(h), a == int64(i); got != want {
				t.Fatalf("eqs(%d)[%d]=%v, want %v", a, i, got, want)
			}
		}
		arithmeticRelease(t, m, eqs)
		arithmeticRelease(t, m, aRefs)
	}
}

func TestBitBlasterMultiplexerRetainsSymbolicITE(t *testing.T) {
	m, engine := arithmeticEngine(t)
	boolSort, err := m.MkBoolSort()
	paramsMust(t, err)
	condition, err := m.MkConst(usymbol.Numerical(811), boolSort)
	paramsMust(t, err)
	gatePin(t, m, condition)
	thenBit, err := m.MkConst(usymbol.Numerical(812), boolSort)
	paramsMust(t, err)
	gatePin(t, m, thenBit)
	elseBit, err := m.MkConst(usymbol.Numerical(813), boolSort)
	paramsMust(t, err)
	gatePin(t, m, elseBit)
	result, err := engine.MkMultiplexer(condition, []ast.ASTHandle{thenBit}, []ast.ASTHandle{elseBit})
	paramsMust(t, err)
	h, err := m.RefGet(result[0])
	paramsMust(t, err)
	kind, err := m.GetDeclKind(h)
	paramsMust(t, err)
	if kind != ast.ASTOpIte {
		t.Fatal("symbolic mux kind", kind)
	}
	arithmeticRelease(t, m, result)
}
