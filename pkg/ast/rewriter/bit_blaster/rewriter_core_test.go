package bit_blaster

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func rewriterFixture(t *testing.T) (*ast.ASTManager, *BitBlasterRewriter) {
	t.Helper()
	m := gateManager(t)
	_, err := m.RegisterArithmeticPlugin()
	paramsMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	paramsMust(t, err)
	r, err := NewBitBlasterRewriter(m, nil)
	paramsMust(t, err)
	t.Cleanup(func() { paramsMust(t, r.Close()) })
	return m, r
}

func TestBitBlasterRewriterGetBitsAndAssemble(t *testing.T) {
	m, r := rewriterFixture(t)
	if got, err := r.Manager(); err != nil || got != m {
		t.Fatal("manager accessor", got, err)
	}
	defaults := DefaultRewriterParams()
	if defaults.MaxSteps != ^uint32(0) || defaults.MaxMemoryMB != ^uint32(0) || !defaults.BlastAdd || !defaults.BlastMul || defaults.BlastFull || defaults.BlastQuant {
		t.Fatal("rewriter defaults", defaults)
	}
	limited := defaults
	limited.MaxMemoryMB = 0
	paramsMust(t, r.UpdateParams(limited))
	if r.blaster.maxMemory != 0 {
		t.Fatal("rewriter memory update", r.blaster.maxMemory)
	}
	paramsMust(t, r.UpdateParams(defaults))
	sort, err := m.BitVectorSort(4)
	paramsMust(t, err)
	value, err := m.MkConst(usymbol.Numerical(1101), sort)
	paramsMust(t, err)
	gatePin(t, m, value)
	bits, err := r.getBits(value)
	paramsMust(t, err)
	if len(bits) != 4 {
		t.Fatal("bit count", len(bits))
	}
	for index, bit := range arithmeticHandles(t, r.blaster, bits) {
		kind, err := m.GetDeclKind(bit)
		paramsMust(t, err)
		if kind != 55 {
			t.Fatalf("bit %d kind %d", index, kind)
		}
	}
	out, err := m.NewRef(ast.ASTHandle{})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.DelRef(&out)) }()
	paramsMust(t, r.setBitVector(bits, out))
	assembled, err := m.RefGet(out)
	paramsMust(t, err)
	if yes, err := m.IsBitVectorOperation(assembled, 56); err != nil || !yes {
		t.Fatal("assembled mkbv", yes, err)
	}
	recovered, err := r.getBits(assembled)
	paramsMust(t, err)
	if got := arithmeticHandles(t, r.blaster, recovered); len(got) != 4 {
		t.Fatal("recovered bit count", len(got))
	} else {
		want := arithmeticHandles(t, r.blaster, bits)
		for i := range got {
			if got[i] != want[i] {
				t.Fatal("mkbv direct bit path", i, got[i], want[i])
			}
		}
	}
	arithmeticRelease(t, m, recovered)
	arithmeticRelease(t, m, bits)
}

func TestBitBlasterRewriterConstantCacheScopes(t *testing.T) {
	m, r := rewriterFixture(t)
	sort, err := m.BitVectorSort(3)
	paramsMust(t, err)
	first, err := m.MkConst(usymbol.Numerical(1111), sort)
	paramsMust(t, err)
	second, err := m.MkConst(usymbol.Numerical(1112), sort)
	paramsMust(t, err)
	gatePin(t, m, first)
	gatePin(t, m, second)
	firstDecl, err := m.GetDecl(first)
	paramsMust(t, err)
	secondDecl, err := m.GetDecl(second)
	paramsMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.DelRef(&out)) }()
	paramsMust(t, r.makeConstant(firstDecl, out))
	firstBits, err := m.RefGet(out)
	paramsMust(t, err)
	paramsMust(t, r.makeConstant(firstDecl, out))
	again, err := m.RefGet(out)
	paramsMust(t, err)
	if firstBits != again {
		t.Fatal("constant cache miss", firstBits, again)
	}
	paramsMust(t, r.Push())
	paramsMust(t, r.makeConstant(secondDecl, out))
	if scopes, err := r.NumScopes(); err != nil || scopes != 1 {
		t.Fatal("scope count", scopes, err)
	}
	cache, err := r.Const2Bits()
	paramsMust(t, err)
	if len(cache) != 2 {
		t.Fatal("cache size before pop", len(cache))
	}
	paramsMust(t, r.Pop(1))
	cache, err = r.Const2Bits()
	paramsMust(t, err)
	if len(cache) != 1 || cache[0].Key != firstDecl || cache[0].Value[0] != firstBits {
		t.Fatal("cache after pop", cache)
	}
}
