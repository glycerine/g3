package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func newUDocProductPairConstantsFailureFixture(t *testing.T) *publicUDocProductFixture {
	t.Helper()
	f := newUDocProductPairFailureFixture(t)
	source := f.head
	symbol, err := f.m.Symbols.Intern("public-doc-product-pair-constants")
	fragmentMust(t, err)
	f.head = f.pin(f.m.MkFuncDecl(symbol, []ast.ASTHandle{f.bits, f.boolean}, f.boolean, ast.ASTFuncDeclInfo{}))
	fragmentMust(t, f.e.RegisterPredicate(f.head))
	body := f.pin(f.m.MkAnd(f.app(source, f.x, f.yes), f.app(source, f.values[1], f.b)))
	formula := f.pin(f.m.MkImplies(body, f.app(f.head, f.x, f.b)))
	fragmentMust(t, f.e.AddRule(formula, "pair-constants"))
	return f
}

func TestDatalogUDocProductsPairConstantsAllocationFailureRetryAndClose(t *testing.T) {
	checkPublicUDocProductAllocationFailureRetryAndClose(t, newUDocProductPairConstantsFailureFixture)
}

func TestDatalogUDocProductsPairConstantsCancellationAndHeldAnswer(t *testing.T) {
	f := newUDocProductPairConstantsFailureFixture(t)
	f.checkRows(lbool.True, [2]uint32{1, 1})
	plan := f.e.udocDatalog.products.program.rules[f.head]
	if plan.pair == nil || len(plan.filters) != 2 || len(plan.pair.leftColumns) != 0 {
		t.Fatal("constant operands did not execute separate filters after a Cartesian product join")
	}
	held, err := f.e.Answer(f.head)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, f.m.DelRef(&held)) }()
	f.m.Limit.Cancel()
	_, cause := f.e.QueryPredicate(f.head)
	f.m.Limit.ResetCancel()
	if cause == nil {
		t.Fatal("constant-filtered product query ignored cancellation")
	}
	stale, err := f.e.Answer(f.head)
	if err == nil || stale != (ast.ASTRef{}) {
		if stale != (ast.ASTRef{}) {
			_ = f.m.DelRef(&stale)
		}
		t.Fatal("cancelled constant-filtered query published a stale answer", err)
	}
	f.checkFormula(held, []ast.ASTHandle{f.values[1], f.yes}, true)
	f.checkRows(lbool.True, [2]uint32{1, 1})
	if f.e.udocDatalog.products.result[f.head] == nil || len(f.e.udocDatalog.products.pending) != 0 {
		t.Fatal("constant-filtered query retry lost actual product storage or pending cleanup")
	}
}
