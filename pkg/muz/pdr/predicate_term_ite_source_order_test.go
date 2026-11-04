package pdr

// Source: pdr_context.cpp:610-616 rewrites, then blasts ITEs without a
// second theory-rewriter pass over the resulting expression.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	tacticcore "github.com/glycerine/g3/pkg/tactic/core"
)

func TestPDRRuleTermITEUsesSourceRewriteOrder(t *testing.T) {
	m, c, rules := classifierContextFixture(t)
	var roots []ast.ASTRef
	defer func() { closureMust(t, closeASTRefs(m, roots)) }()
	own := func(h ast.ASTHandle, e error) ast.ASTHandle {
		closureMust(t, e)
		r, x := m.NewRef(h)
		closureMust(t, x)
		roots = append(roots, r)
		return h
	}
	boolean := own(m.MkBoolSort())
	arithmetic, err := m.ArithmeticFamilyID()
	closureMust(t, err)
	realSort := own(m.MkFamilySort(arithmetic, 0, nil))
	name, err := m.Symbols.Intern("blast-source-order-p")
	closureMust(t, err)
	predicate := own(m.MkFuncDecl(name, []ast.ASTHandle{realSort, boolean}, boolean, ast.ASTFuncDeclInfo{}))
	closureMust(t, rules.RegisterPredicate(predicate))
	x := own(m.MkVar(0, realSort))
	cond := own(m.MkVar(1, boolean))
	one := own(m.ArithmeticRealNumeralText("1"))
	head := own(m.MkApp(predicate, []ast.ASTHandle{x, cond}))
	choice := own(m.MkIte(cond, x, one))
	tail := own(m.MkEq(choice, x))
	set, err := base.NewRuleSet(rules)
	closureMust(t, err)
	defer func() { closureMust(t, set.Close()) }()
	rule, err := rules.NewRule(head, []ast.ASTHandle{tail}, nil, "blast-source-order")
	closureMust(t, err)
	closureMust(t, set.Add(rule))
	closureMust(t, rule.Close())
	closureMust(t, c.Initialize(set))
	if mode := c.owner.contexts.arithmeticMode(); mode != smtcore.SMTArithmeticDifferenceLogic {
		t.Fatal("test did not reach difference-logic ITE blast", mode)
	}
	transformer := c.transformers[predicate]
	if transformer == nil || len(transformer.rules) != 1 {
		t.Fatal("normalized rule unavailable")
	}
	ground := make([]ast.ASTHandle, 2)
	for i, d := range transformer.signature {
		d, err = c.owner.OldToNew(d, 0)
		closureMust(t, err)
		ground[i] = own(m.MkApp(d, nil))
	}
	groundedChoice := own(m.MkIte(ground[1], ground[0], one))
	groundedTail := own(m.MkEq(groundedChoice, ground[0]))
	cfg := c.owner.contexts.config
	theory, err := rewriter.NewTheoryRewriter(m, cfg.Params, cfg.Globals, nil)
	closureMust(t, err)
	defer func() { closureMust(t, theory.Close()) }()
	preRef, err := m.NewRef(ast.ASTHandle{})
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&preRef)) }()
	closureMust(t, theory.Rewrite(groundedTail, preRef))
	pre, err := m.RefGet(preRef)
	closureMust(t, err)
	expectedRef, err := m.NewRef(ast.ASTHandle{})
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&expectedRef)) }()
	closureMust(t, tacticcore.BlastTermITEInto(m, pre, expectedRef))
	expected, err := m.RefGet(expectedRef)
	closureMust(t, err)
	postRef, err := m.NewRef(ast.ASTHandle{})
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&postRef)) }()
	closureMust(t, theory.Rewrite(expected, postRef))
	post, err := m.RefGet(postRef)
	closureMust(t, err)
	if expected == post {
		t.Fatal("oracle does not distinguish the extra theory rewrite")
	}
	if got := transformer.rules[0].transition; got != expected {
		t.Fatalf("rule transition differs from source blast result: got %v want %v (extra rewrite %v)", got, expected, post)
	}
}
