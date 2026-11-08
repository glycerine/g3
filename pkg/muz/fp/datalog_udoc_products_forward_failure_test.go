package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func newUDocProductForwardFailureFixture(t *testing.T) *publicUDocProductFixture {
	t.Helper()
	f := newMixedGroundUDocProductFailureFixture(t)
	source := f.head
	name, err := f.m.Symbols.Intern("public-doc-product-forward-result")
	fragmentMust(t, err)
	f.head = f.pin(f.m.MkFuncDecl(name, []ast.ASTHandle{f.bits, f.boolean}, f.boolean, ast.ASTFuncDeclInfo{}))
	fragmentMust(t, f.e.RegisterPredicate(f.head))
	formula := f.pin(f.m.MkImplies(f.app(source, f.x, f.b), f.app(f.head, f.x, f.b)))
	fragmentMust(t, f.e.addRuleEager(formula, "forward-product-result"))
	return f
}

func TestDatalogUDocProductsForwardAllocationFailureRetryAndClose(t *testing.T) {
	// The original exhaustive public campaign verifies native failure causes,
	// answer invalidation, exact AST counts, repeated retry and zero final
	// budget. This fixture makes its published result an ordinary forwarded
	// product, so it also reaches the newly admitted target's clone/union path.
	checkPublicUDocProductAllocationFailureRetryAndClose(t, newUDocProductForwardFailureFixture)
}

func TestDatalogUDocProductsForwardCancellationAndHeldAnswer(t *testing.T) {
	f := newUDocProductForwardFailureFixture(t)
	f.checkRows(lbool.True, [2]uint32{1, 1})
	held, err := f.e.Answer(f.head)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, f.m.DelRef(&held)) }()
	f.m.Limit.Cancel()
	_, cause := f.e.QueryPredicate(f.head)
	f.m.Limit.ResetCancel()
	if cause == nil {
		t.Fatal("ordinary product forwarding ignored cancellation")
	}
	stale, err := f.e.Answer(f.head)
	if err == nil || stale != (ast.ASTRef{}) {
		if stale != (ast.ASTRef{}) {
			_ = f.m.DelRef(&stale)
		}
		t.Fatal("cancelled forwarding published a stale answer", err)
	}
	f.checkFormula(held, []ast.ASTHandle{f.values[1], f.yes}, true)
	f.checkRows(lbool.True, [2]uint32{1, 1})
	if f.e.udocDatalog.products.result[f.head] == nil || len(f.e.udocDatalog.products.pending) != 0 {
		t.Fatal("forwarding retry lost actual product storage or pending cleanup")
	}
}
