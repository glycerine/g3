package smtcore

import (
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Publicly registered nodes remain in the checker when their neighbouring
// solver-owned applications leave a scope. Both indexes preserve their order.
func TestSMTPopWithBorrowedApplicationsOfSameDeclaration(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "borrowed-declaration", 6)
	var terms []ast.ASTHandle
	for _, arg := range f.base {
		term, err := f.m.MkApp(f.unary, []ast.ASTHandle{arg})
		astTestMust(t, err)
		astTestPin(t, f.m, term)
		terms = append(terms, term)
	}
	ctx := f.s.context
	borrow := func(i int, suppress bool) *SMTEnode {
		t.Helper()
		n, err := NewSMTEnode(f.m, terms[i], ctx.enodes.get, SMTEnodeOptions{SuppressArguments: suppress})
		astTestMust(t, err)
		astTestMust(t, ctx.RegisterEnode(n, true))
		return n
	}
	first := borrow(0, false)
	astTestMust(t, f.s.PushScope())
	astTestMust(t, f.s.Internalize(terms[1], false))
	second := borrow(2, false)
	hidden := borrow(3, true)
	third := borrow(4, false)
	astTestMust(t, f.s.Internalize(terms[5], false))
	it := newSMTMAMInterpreter(f.s, nil, false)
	if got := it.enodesOf(f.unary); len(got) != 5 {
		t.Fatal("unexpected matching candidates before pop", len(got))
	}
	astTestMust(t, f.s.PopScopes(1))
	want := []*SMTEnode{first, second, third}
	if !slices.Equal(it.enodesOf(f.unary), want) {
		t.Fatal("pop lost or reordered matching candidates")
	}
	if len(ctx.registeredEnodes) < 4 || !slices.Equal(ctx.registeredEnodes[len(ctx.registeredEnodes)-4:], []*SMTEnode{first, second, hidden, third}) {
		t.Fatal("pop reordered borrowed nodes")
	}
	for _, i := range []int{1, 5} {
		if ctx.enodes.get(terms[i]) != nil {
			t.Fatal("retired application remains registered", i)
		}
	}
	for _, n := range []*SMTEnode{first, second, hidden, third} {
		if ctx.enodes.get(n.Owner()) != n {
			t.Fatal("borrowed application lost its registration")
		}
	}
	astTestMust(t, f.s.CheckInvariant())
	astTestMust(t, f.s.Close())
	for _, n := range []*SMTEnode{first, second, hidden, third} {
		astTestMust(t, n.Delete(false))
	}
}
