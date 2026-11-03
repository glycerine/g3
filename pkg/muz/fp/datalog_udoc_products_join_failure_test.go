package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func newUDocProductPairFailureFixture(t *testing.T) *publicUDocProductFixture {
	t.Helper()
	f := newMixedGroundUDocProductFailureFixture(t)
	source := f.head
	declare := func(name string) ast.ASTHandle {
		symbol, err := f.m.Symbols.Intern(name)
		fragmentMust(t, err)
		declaration := f.pin(f.m.MkFuncDecl(symbol, []ast.ASTHandle{f.bits, f.boolean}, f.boolean, ast.ASTFuncDeclInfo{}))
		fragmentMust(t, f.e.RegisterPredicate(declaration))
		return declaration
	}
	second := declare("public-doc-product-pair-copy")
	f.head = declare("public-doc-product-pair-result")
	firstApp, secondApp := f.app(source, f.x, f.b), f.app(second, f.x, f.b)
	forward := f.pin(f.m.MkImplies(firstApp, secondApp))
	fragmentMust(t, f.e.AddRule(forward, "pair-copy"))
	body := f.pin(f.m.MkAnd(firstApp, secondApp))
	formula := f.pin(f.m.MkImplies(body, f.app(f.head, f.x, f.b)))
	fragmentMust(t, f.e.AddRule(formula, "pair-result"))
	return f
}

func TestDatalogUDocProductsPairAllocationFailureRetryAndClose(t *testing.T) {
	// Each ordinal executes the actual pair primitive and target union. The
	// shared campaign verifies exact live AST counts, original failure causes,
	// invalidated answers, repeated retries, idempotent close and zero budget.
	checkPublicUDocProductAllocationFailureRetryAndClose(t, newUDocProductPairFailureFixture)
}

func TestDatalogUDocProductsPairCancellationAndHeldAnswer(t *testing.T) {
	f := newUDocProductPairFailureFixture(t)
	f.checkRows(lbool.True, [2]uint32{1, 1})
	if f.e.udocDatalog.products.program.rules[f.head].pair == nil {
		t.Fatal("pair fixture did not execute two-product dispatch")
	}
	held, err := f.e.Answer(f.head)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, f.m.DelRef(&held)) }()
	f.m.Limit.Cancel()
	_, cause := f.e.QueryPredicate(f.head)
	f.m.Limit.ResetCancel()
	if cause == nil {
		t.Fatal("paired product query ignored cancellation")
	}
	stale, err := f.e.Answer(f.head)
	if err == nil || stale != (ast.ASTRef{}) {
		if stale != (ast.ASTRef{}) {
			_ = f.m.DelRef(&stale)
		}
		t.Fatal("cancelled paired query published a stale answer", err)
	}
	f.checkFormula(held, []ast.ASTHandle{f.values[1], f.yes}, true)
	f.checkRows(lbool.True, [2]uint32{1, 1})
	if f.e.udocDatalog.products.result[f.head] == nil || len(f.e.udocDatalog.products.pending) != 0 {
		t.Fatal("paired query retry lost actual product storage or pending cleanup")
	}
}
