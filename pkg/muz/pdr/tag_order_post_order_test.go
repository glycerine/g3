package pdr

// Source: pred_transformer::m_tag2rule walks (find_rule, mk_assumptions,
// get_propagation_formula) and the farkas_learner::get_lemmas todo loop in
// src/muz/pdr/pdr_context.cpp and pdr_farkas_learner.cpp.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// AUDIT.md FUZZ_F-F3: the tagged rules come back in obj_map table order and
// untagged rules are skipped.
func TestPDRTagTableRulesFollowObjMapOrder(t *testing.T) {
	manager, _, _ := classifierContextFixture(t)
	boolean, err := manager.MkBoolSort()
	closureMust(t, err)
	p := &PredicateTransformer{manager: manager}
	want := ast.NewObjMap(manager)
	byTag := make(map[ast.ASTHandle]*normalizedRule)
	for i := 0; i < 6; i++ {
		rule := &normalizedRule{}
		if i != 2 {
			tag, err := manager.MkFreshConst("pdr-tag-order", boolean)
			closureMust(t, err)
			rule.tag = tag
			_, err = want.InsertIfNotThere(tag)
			closureMust(t, err)
			byTag[tag] = rule
		}
		p.rules = append(p.rules, rule)
	}
	got, err := p.tagTableRules()
	closureMust(t, err)
	entries := want.Entries()
	if len(got) != len(entries) {
		t.Fatalf("got %d rules, want %d", len(got), len(entries))
	}
	for i, entry := range entries {
		if got[i] != byTag[entry.Key] {
			t.Fatalf("rule %d is not the obj_map entry %d", i, i)
		}
	}
}

// AUDIT.md FUZZ_F-F5: get_lemmas pushes every unvisited parent and examines
// todo.back() first, so the last parent's subtree is numbered first.
func TestPDRFarkasProofPostOrderLastParentFirst(t *testing.T) {
	manager, _, _ := classifierContextFixture(t)
	closureMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	defer func() { closureMust(t, manager.ToggleProofMode(ast.ASTProofDisabled)) }()
	boolean, err := manager.MkBoolSort()
	closureMust(t, err)
	constant := func(name string) ast.ASTHandle {
		symbol, err := manager.Symbols.Intern(name)
		closureMust(t, err)
		h, err := manager.MkConst(symbol, boolean)
		closureMust(t, err)
		return h
	}
	a, b, c := constant("post-order-a"), constant("post-order-b"), constant("post-order-c")
	ab, err := manager.MkFamilyApp(0, ast.ASTOpIff, nil, []ast.ASTHandle{a, b})
	closureMust(t, err)
	bc, err := manager.MkFamilyApp(0, ast.ASTOpIff, nil, []ast.ASTHandle{b, c})
	closureMust(t, err)
	pab, err := manager.MkProofHypothesis(ab)
	closureMust(t, err)
	pbc, err := manager.MkProofHypothesis(bc)
	closureMust(t, err)
	transitivity, err := manager.MkProofTransitivity(pab, pbc)
	closureMust(t, err)
	pa, err := manager.MkProofHypothesis(a)
	closureMust(t, err)
	root, err := manager.MkProofModusPonens(pa, transitivity)
	closureMust(t, err)
	f := &FarkasLearner{manager: manager}
	order, err := f.proofPostOrder(root)
	closureMust(t, err)
	want := []ast.ASTHandle{pbc, pab, transitivity, pa, root}
	if len(order) != len(want) {
		t.Fatalf("order has %d proofs, want %d", len(order), len(want))
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("post-order position %d differs", i)
		}
	}
}
