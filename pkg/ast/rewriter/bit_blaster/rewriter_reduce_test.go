package bit_blaster

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func rewriterConstant(t *testing.T, r *BitBlasterRewriter, value int64, width int) ast.ASTRef {
	t.Helper()
	bits, err := r.blaster.NumToBits(bbSmall(value), width)
	paramsMust(t, err)
	handles := arithmeticHandles(t, r.blaster, bits)
	term, err := r.manager.MkBitVectorFromBits(handles)
	paramsMust(t, err)
	arithmeticRelease(t, r.manager, bits)
	ref, err := r.manager.NewRef(term)
	paramsMust(t, err)
	return ref
}

func rewriterValue(t *testing.T, r *BitBlasterRewriter, term ast.ASTHandle) int64 {
	t.Helper()
	bits, err := r.getBits(term)
	paramsMust(t, err)
	defer arithmeticRelease(t, r.manager, bits)
	return arithmeticValue(t, r.blaster, bits)
}

func TestBitBlasterRewriterCircuitReductions(t *testing.T) {
	m, r := rewriterFixture(t)
	aRef := rewriterConstant(t, r, 3, 4)
	bRef := rewriterConstant(t, r, 5, 4)
	cRef := rewriterConstant(t, r, 7, 4)
	defer func() {
		paramsMust(t, m.DelRef(&cRef))
		paramsMust(t, m.DelRef(&bRef))
		paramsMust(t, m.DelRef(&aRef))
	}()
	a, err := m.RefGet(aRef)
	paramsMust(t, err)
	b, err := m.RefGet(bRef)
	paramsMust(t, err)
	c, err := m.RefGet(cRef)
	paramsMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.DelRef(&out)) }()

	paramsMust(t, r.reduceBinary(a, b, out, r.blaster.MkAdder))
	got, err := m.RefGet(out)
	paramsMust(t, err)
	if value := rewriterValue(t, r, got); value != 8 {
		t.Fatal("binary add", value)
	}
	paramsMust(t, r.reduceAC(ast.SliceArguments([]ast.ASTHandle{a, b, c}), out, r.blaster.MkAdder))
	got, err = m.RefGet(out)
	paramsMust(t, err)
	if value := rewriterValue(t, r, got); value != 15 {
		t.Fatal("AC add", value)
	}
	paramsMust(t, r.reduceUnary(a, out, r.blaster.MkNot))
	got, err = m.RefGet(out)
	paramsMust(t, err)
	if value := rewriterValue(t, r, got); value != 12 {
		t.Fatal("unary not", value)
	}
	paramsMust(t, r.reducePredicate(a, b, out, r.blaster.MkUnsignedLE))
	got, err = m.RefGet(out)
	paramsMust(t, err)
	if !m.IsTrue(got) {
		t.Fatal("unsigned predicate")
	}

	truth, err := m.MkTrue()
	paramsMust(t, err)
	paramsMust(t, r.reduceITE(truth, a, b, out))
	got, err = m.RefGet(out)
	paramsMust(t, err)
	if value := rewriterValue(t, r, got); value != 3 {
		t.Fatal("ITE", value)
	}
	paramsMust(t, r.reduceConcat(ast.SliceArguments([]ast.ASTHandle{a, b}), out))
	got, err = m.RefGet(out)
	paramsMust(t, err)
	if value := rewriterValue(t, r, got); value != 0x35 {
		t.Fatal("concat", value)
	}
	paramsMust(t, r.reduceExtract(1, 3, b, out))
	got, err = m.RefGet(out)
	paramsMust(t, err)
	if value := rewriterValue(t, r, got); value != 2 {
		t.Fatal("extract", value)
	}
	paramsMust(t, r.reduceSignExtend(2, c, out))
	got, err = m.RefGet(out)
	paramsMust(t, err)
	if value := rewriterValue(t, r, got); value != 7 {
		t.Fatal("positive sign extend", value)
	}
	paramsMust(t, r.reduceReduction(a, out, false))
	got, err = m.RefGet(out)
	paramsMust(t, err)
	if value := rewriterValue(t, r, got); value != 1 {
		t.Fatal("reduction or", value)
	}
}
