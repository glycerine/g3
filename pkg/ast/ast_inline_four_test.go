package ast

import (
	"math"
	"testing"
	"unsafe"

	"github.com/glycerine/g3/pkg/arena"
)

func TestInlineFourthOperandIdentity(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	var terms [5]ASTHandle
	for i := range terms {
		terms[i], e = m.MkVar(uint32(i), sort)
		astTestMust(t, e)
		astTestPin(t, m, terms[i])
	}
	decl, e := m.MkFuncDecl(astTestSymbol(t, m, "four"), []ASTHandle{sort, sort, sort, sort}, sort, ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, decl)
	a, e := m.MkApp(decl, terms[:4])
	astTestMust(t, e)
	astTestPin(t, m, a)
	bArgs := []ASTHandle{terms[0], terms[1], terms[2], terms[4]}
	b, e := m.MkApp(decl, bArgs)
	astTestMust(t, e)
	astTestPin(t, m, b)
	for _, pair := range []struct {
		term ASTHandle
		args []ASTHandle
	}{{a, terms[:4]}, {b, bArgs}} {
		found, e := m.MkApp(decl, pair.args)
		astTestMust(t, e)
		if found != pair.term || a == b {
			t.Fatal("fourth operand identity", found)
		}
		for i, want := range pair.args {
			got, e := m.GetArg(found, uint32(i))
			astTestMust(t, e)
			if got != want {
				t.Fatal("argument", i, got, want)
			}
		}
		hash, err := m.nodeHash(astNodeRef{found.slot, found.generation})
		if err != nil || hash != m.nodeAt(found.slot).hash {
			t.Fatal("structural hash", hash, err)
		}
	}
	// A fault-injected constructor uses the ordinary arena payload even
	// when the same arity fits inline in a release build.
	m.Budget().FailAfter(math.MaxInt64)
	observed, err := m.MkApp(decl, []ASTHandle{terms[4], terms[2], terms[1], terms[0]})
	m.Budget().FailAfter(-1)
	astTestMust(t, err)
	astTestPin(t, m, observed)
	if m.appAt(observed.slot).args == (arena.SmallBytesRef{}) {
		t.Fatal("armed constructor skipped operand payload")
	}
	if !astValidateHandles && !ASTSourceDebug && m.appAt(a.slot).args != (arena.SmallBytesRef{}) {
		t.Fatal("four operands allocated a separate payload")
	}
	if unsafe.Sizeof(astNodeSlot{}) != 128 {
		t.Fatal("node slot", unsafe.Sizeof(astNodeSlot{}))
	}
}
