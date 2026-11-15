package pdr

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// AUDIT.md muz_pdr_02-F7: the pred_transformer constructor builds only its
// prop solver and reachable cache; init_sig creates the signature (and its
// mux tuple) on the first sig(i)/sig_size() use.
func TestPredicateTransformerSignatureIsLazy(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	defer closeBase()
	defer func() { closureMust(t, owner.Close()) }()
	closureMust(t, transformer.Close())
	one, err := m.ArithmeticIntegerNumeralText("1")
	closureMust(t, err)
	integer, err := m.GetSort(one)
	closureMust(t, err)
	boolean, err := m.MkBoolSort()
	closureMust(t, err)
	name, err := m.Symbols.Intern("lazy_signature_p")
	closureMust(t, err)
	head, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	closureMust(t, err)
	before, err := owner.mux.PrimaryPredicates()
	closureMust(t, err)
	lazy, err := NewPredicateTransformer(owner, head, ReachableHashCache)
	closureMust(t, err)
	defer func() { closureMust(t, lazy.Close()) }()
	after, err := owner.mux.PrimaryPredicates()
	closureMust(t, err)
	if len(after) != len(before) {
		t.Fatal("constructor created the signature", len(before), len(after))
	}
	signature, err := lazy.Signature()
	closureMust(t, err)
	if len(signature) != 1 {
		t.Fatal("signature size", len(signature))
	}
	after, err = owner.mux.PrimaryPredicates()
	closureMust(t, err)
	if len(after) != len(before)+1 {
		t.Fatal("first sig use did not create the mux tuple", len(before), len(after))
	}
}
