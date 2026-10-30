package spacer

// Source: src/muz/spacer/spacer_context.cpp context::is_reachable queues
// each successive derivation child before recursively confirming it.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestSpacerConcreteConfirmationQueuesSuccessiveSiblings(t *testing.T) {
	for _, confirmed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unconfirmed", true: "confirmed"}[confirmed], func(t *testing.T) {

			f := newContextAnswerFixture(t, false)
			c, m := f.context, f.context.manager
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("confirmation-root")
			must(err)
			rootDecl, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(f.manager.RegisterPredicate(rootDecl))
			must(f.manager.RegisterPredicate(f.other))
			rootHead, err := m.MkApp(rootDecl, nil)
			must(err)
			firstHead, err := f.rule.Head()
			must(err)
			secondHead, err := m.MkApp(f.other, nil)
			must(err)
			rootRule, err := f.manager.NewRule(rootHead, []ast.ASTHandle{firstHead, secondHead}, nil, "confirmation-root-rule")
			must(err)
			defer func() { must(rootRule.Close()) }()
			secondTail := []ast.ASTHandle{secondHead}
			if confirmed {
				secondTail = nil
			}
			secondRule, err := f.manager.NewRule(secondHead, secondTail, nil, "confirmation-sibling")
			must(err)
			defer func() { must(secondRule.Close()) }()
			must(f.rules.Add(rootRule))
			must(f.rules.Add(secondRule))
			must(c.UpdateRules(f.rules))
			must(c.SetQuery(rootDecl))
			truth, err := m.MkTrue()
			must(err)
			root, err := c.transformers[rootDecl].NewProofObligation(nil, 2, 0, truth, nil)
			must(err)
			md, err := model.NewModel(m)
			must(err)
			defer func() { must(md.Close()) }()
			first, err := c.transformers[rootDecl].CreateDerivationChild(root, rootRule, md.Core(), []bool{false, false}, false)
			must(err)
			if first == nil || first.transformer != c.transformers[f.query] {
				t.Fatal("expected the reachable first premise")
			}
			must(c.queue.SetRoot(root))
			must(c.queue.Push(first))
			reachable, err := c.queue.confirmConcreteReachability(first)
			must(err)
			if reachable != confirmed {
				t.Fatal("confirmation result disagrees with the sibling reach facts", reachable, confirmed)
			}
			closed, err := first.IsClosed()
			must(err)
			if !closed {
				t.Fatal("confirmed first premise did not close")
			}
			next, err := c.queue.Top()
			must(err)
			if next == nil || next == first || next.transformer != c.transformers[f.other] {
				t.Fatal("unconfirmed next sibling was not queued after the first premise closed")
			}
			if closed, err := next.IsClosed(); err != nil || closed != confirmed {
				t.Fatal("queued sibling closed state", closed, confirmed, err)
			}
			if _, found, err := next.Derivation(); err != nil || found == confirmed {
				t.Fatal("queued sibling derivation ownership", found, confirmed, err)
			}

		})
	}
}
