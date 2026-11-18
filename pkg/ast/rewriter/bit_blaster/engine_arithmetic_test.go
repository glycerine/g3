package bit_blaster

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func arithmeticEngine(t *testing.T) (*ast.ASTManager, *Engine) {
	t.Helper()
	m := gateManager(t)
	_, err := m.RegisterArithmeticPlugin()
	paramsMust(t, err)
	family, err := m.RegisterBitVectorPlugin()
	paramsMust(t, err)
	config, err := NewConfig(m, family, SMTBitBlasterParams{ExtGates: true})
	paramsMust(t, err)
	engine, err := NewEngine(config)
	paramsMust(t, err)
	return m, engine
}

func arithmeticHandles(t *testing.T, e *Engine, refs []ast.ASTRef) []ast.ASTHandle {
	t.Helper()
	result, err := e.handles(refs)
	paramsMust(t, err)
	return result
}

func arithmeticRelease(t *testing.T, m *ast.ASTManager, refs []ast.ASTRef) {
	t.Helper()
	paramsMust(t, releaseRefs(m, refs))
}

func arithmeticValue(t *testing.T, e *Engine, refs []ast.ASTRef) int64 {
	t.Helper()
	value, ok := bbNumeral(t, e, arithmeticHandles(t, e, refs))
	if !ok {
		t.Fatal("constant arithmetic produced a symbolic bit")
	}
	return value
}

func TestBitBlasterArithmeticExhaustiveConstants(t *testing.T) {
	m, engine := arithmeticEngine(t)
	for width := 1; width <= 5; width++ {
		limit := int64(1 << width)
		mask := limit - 1
		for a := int64(0); a < limit; a++ {
			aRefs, err := engine.NumToBits(bbSmall(a), width)
			paramsMust(t, err)
			aBits := arithmeticHandles(t, engine, aRefs)

			negative, err := engine.MkNeg(aBits)
			paramsMust(t, err)
			if got, want := arithmeticValue(t, engine, negative), (-a)&mask; got != want {
				t.Fatalf("width %d: -%d = %d, want %d", width, a, got, want)
			}
			arithmeticRelease(t, m, negative)

			for b := int64(0); b < limit; b++ {
				bRefs, err := engine.NumToBits(bbSmall(b), width)
				paramsMust(t, err)
				bBits := arithmeticHandles(t, engine, bRefs)

				sum, err := engine.MkAdder(aBits, bBits)
				paramsMust(t, err)
				if got, want := arithmeticValue(t, engine, sum), (a+b)&mask; got != want {
					t.Fatalf("width %d: %d+%d = %d, want %d", width, a, b, got, want)
				}
				arithmeticRelease(t, m, sum)

				difference, carry, err := engine.MkSubtracter(aBits, bBits)
				paramsMust(t, err)
				if got, want := arithmeticValue(t, engine, difference), (a-b)&mask; got != want {
					t.Fatalf("width %d: %d-%d = %d, want %d", width, a, b, got, want)
				}
				carryHandle, err := m.RefGet(carry)
				paramsMust(t, err)
				if got, want := m.IsTrue(carryHandle), a >= b; got != want {
					t.Fatalf("width %d: carry(%d-%d) = %v, want %v", width, a, b, got, want)
				}
				arithmeticRelease(t, m, difference)
				paramsMust(t, m.DelRef(&carry))
				arithmeticRelease(t, m, bRefs)
			}
			arithmeticRelease(t, m, aRefs)
		}
	}
}

func TestBitBlasterNumeralAndArithmeticDomains(t *testing.T) {
	m, engine := arithmeticEngine(t)
	bits, err := engine.NumToBits(bbSmall(5), 4)
	paramsMust(t, err)
	if value, ok := bbNumeral(t, engine, arithmeticHandles(t, engine, bits)); !ok || value != 5 {
		t.Fatal("little-endian numeral round trip", value, ok)
	}
	if engine.IsMinusOne(arithmeticHandles(t, engine, bits)) {
		t.Fatal("5 is not all ones at width 4")
	}
	arithmeticRelease(t, m, bits)

	if _, err = engine.NumToBits(bbSmall(-1), 4); err == nil {
		t.Fatal("negative numeral accepted")
	}
	if _, err = engine.MkAdder(nil, nil); err == nil {
		t.Fatal("zero-width adder accepted")
	}
	if _, err = engine.MkNeg(nil); err == nil {
		t.Fatal("zero-width negation accepted")
	}
}
