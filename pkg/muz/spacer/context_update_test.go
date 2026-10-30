package spacer

// Source: src/muz/spacer/spacer_context.cpp context::update_rules and
// spacer_context.h pred_transformer::frames::inherit_frames.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerUpdateRulesCopiesFramesAndClearsSearchState(t *testing.T) {
	f := newContextAnswerFixture(t, true)
	c, m := f.context, f.context.manager
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	status, err := c.Solve(0, 3)
	must(err)
	if status != lbool.False {
		t.Fatal("recursive predicate should be unreachable", status)
	}
	previous := c.transformers[f.query]
	boolean, err := m.MkBoolSort()
	must(err)
	symbol, err := m.Symbols.Intern("update-rules-quantified-property")
	must(err)
	property, err := m.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	variable, err := m.MkVar(0, boolean)
	must(err)
	body, err := m.MkApp(property, []ast.ASTHandle{variable})
	must(err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
	must(err)
	lemma, err := NewLemma(c.owner, quantifier, 4)
	must(err)
	truth, err := m.MkTrue()
	must(err)
	must(lemma.AddBindings([]ast.ASTHandle{truth}))
	_, err = previous.frames.AddLemma(lemma)
	must(err)
	type savedLemma struct {
		body     ast.ASTHandle
		level    uint32
		bindings []ast.ASTHandle
		original *Lemma
	}
	var saved []savedLemma
	for _, item := range previous.frames.lemmas {
		body, err := item.Body()
		must(err)
		level, err := item.Level()
		must(err)
		bindings, err := item.bindingHandles()
		must(err)
		saved = append(saved, savedLemma{body, level, bindings, item})
	}
	if err = c.UpdateRules(nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("invalid update accepted", err)
	}
	if c.transformers[f.query] != previous || previous.open() != nil {
		t.Fatal("invalid update destroyed existing transformers")
	}
	must(c.UpdateRules(f.rules))
	current := c.transformers[f.query]
	if current == previous || previous.open() == nil {
		t.Fatal("update must replace and close previous predicate transformers")
	}
	if c.query != nil || c.queryDecl != f.query || c.solved || c.lastResult != lbool.Undef || c.inductive != 0 || c.queue.root != nil || len(c.queue.items) != 0 {
		t.Fatal("update left stale query, answer, or proof-obligation state")
	}
	if len(current.frames.lemmas) != len(saved) {
		t.Fatal("inherited frame size", len(current.frames.lemmas), len(saved))
	}
	for i, expected := range saved {
		item := current.frames.lemmas[i]
		if item == expected.original {
			t.Fatal("update shared mutable lemma objects with destroyed frames")
		}
		if _, err := expected.original.Body(); err == nil {
			t.Fatal("old frame lemma survived transformer destruction")
		}
		body, err := item.Body()
		must(err)
		level, err := item.Level()
		must(err)
		bindings, err := item.bindingHandles()
		must(err)
		if body != expected.body || level != expected.level || len(bindings) != len(expected.bindings) {
			t.Fatal("inherited lemma body, level, or bindings changed", i)
		}
		for j := range bindings {
			if bindings[j] != expected.bindings[j] {
				t.Fatal("inherited binding changed", i, j)
			}
		}
	}
	status, err = c.Solve(0, 3)
	must(err)
	if status != lbool.False {
		t.Fatal("inherited invariants no longer block the recursive query", status)
	}
	must(c.UpdateRules(f.rules))
	must(c.SetQuery(f.query))
	status, err = c.Solve(0, 3)
	must(err)
	if status != lbool.False {
		t.Fatal("repeated rule update lost the inherited invariant", status)
	}
}

func TestSpacerUpdateRulesReplacesPredicateSetAndInitialReachFacts(t *testing.T) {
	f := newContextAnswerFixture(t, true)
	c, m := f.context, f.context.manager
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	status, err := c.Solve(0, 3)
	must(err)
	if status != lbool.False {
		t.Fatal("old recursive query status", status)
	}
	previous := c.transformers[f.query]
	must(f.manager.RegisterPredicate(f.other))
	head, err := m.MkApp(f.other, nil)
	must(err)
	rule, err := f.manager.NewRule(head, nil, nil, "replacement-fact")
	must(err)
	defer func() { must(rule.Close()) }()
	set, err := base.NewRuleSet(f.manager)
	must(err)
	defer func() { must(set.Close()) }()
	must(set.Add(rule))
	must(c.UpdateRules(set))
	if _, present := c.transformers[f.query]; present || previous.open() == nil {
		t.Fatal("removed predicate remained in the replacement context")
	}
	replacement := c.transformers[f.other]
	if replacement == nil || len(replacement.reachFacts) != 1 || len(replacement.frames.lemmas) != 0 {
		t.Fatal("new predicate did not receive fresh initial reach facts")
	}
	must(c.SetQuery(f.other))
	status, err = c.Solve(0, 1)
	must(err)
	if status != lbool.True {
		t.Fatal("new fact predicate should be reachable", status)
	}
	empty, err := base.NewRuleSet(f.manager)
	must(err)
	defer func() { must(empty.Close()) }()
	must(c.UpdateRules(empty))
	status, err = c.Solve(0, 1)
	must(err)
	if status != lbool.False || c.query != nil {
		t.Fatal("source solve_core must return false when the selected predicate was removed", status)
	}

}
