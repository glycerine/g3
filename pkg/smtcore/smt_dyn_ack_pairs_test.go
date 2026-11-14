package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestDynAckPairOccurrencesLiveGCAndOwnedApps(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkSort(symbol.Numerical(801), ast.ASTSortInfo{})
	astTestMust(t, e)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		astTestMust(t, m.IncRef(h))
		t.Cleanup(func() { astTestMust(t, m.DecRef(h)) })
		return h
	}
	args := make([]ast.ASTHandle, 4)
	for i := range args {
		args[i], e = m.MkConst(symbol.Numerical(uint32(802+i)), sort)
		astTestMust(t, e)
		args[i] = pin(args[i])
	}
	decl, e := m.MkFuncDecl(symbol.Numerical(806), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	apps := make([]ast.ASTHandle, 4)
	base := make(map[ast.ASTHandle]uint32)
	for i, arg := range args {
		apps[i], e = m.MkApp(decl, []ast.ASTHandle{arg})
		astTestMust(t, e)
		apps[i] = pin(apps[i])
		base[apps[i]], e = m.GetRefCount(apps[i])
		astTestMust(t, e)
	}
	state, e := newDynAckPairState(m, 3, .75)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, state.close()) })
	for i := 0; i < 5; i++ {
		astTestMust(t, state.congruenceEvent(apps[1], apps[0]))
	}
	for i := 0; i < 4; i++ {
		astTestMust(t, state.congruenceEvent(apps[2], apps[1]))
	}
	for i := 0; i < 2; i++ {
		astTestMust(t, state.congruenceEvent(apps[3], apps[2]))
	}
	first, second, third := dynAckPair{apps[0], apps[1]}, dynAckPair{apps[1], apps[2]}, dynAckPair{apps[2], apps[3]}
	if len(state.apps) != 3 || len(state.toInstantiate) != 2 || state.apps[0].key != first || state.counts[first] != 5 || state.counts[second] != 4 || state.counts[third] != 2 {
		t.Fatal("source canonicalization, occurrence counts or threshold queue", state.apps, state.counts, state.toInstantiate)
	}
	if e := state.congruenceEvent(apps[0], apps[0]); e != ast.ErrASTDomain {
		t.Fatal("cg_eh distinct-app precondition", e)
	}
	eq1, e := m.MkEq(args[0], args[1])
	astTestMust(t, e)
	eq2, e := m.MkEq(args[2], args[3])
	astTestMust(t, e)
	astTestMust(t, state.congruenceEvent(pin(eq1), pin(eq2)))
	if len(state.apps) != 3 {
		t.Fatal("basic equality applications must be ignored")
	}
	astTestMust(t, state.gc())
	if len(state.apps) != 2 || len(state.toInstantiate) != 2 || state.toInstantiate[0] != first || state.toInstantiate[1] != second || state.counts[first] != 3 || state.counts[second] != 3 {
		t.Fatal("decay must compact dead rows and keep stable candidate order", state.apps, state.counts, state.toInstantiate)
	}
	if _, found := state.counts[third]; found {
		t.Fatal("decayed singleton remained in pair map")
	}
	astTestMust(t, state.markInstantiated(first))
	astTestMust(t, state.congruenceEvent(apps[1], apps[0]))
	if _, found := state.counts[first]; found {
		t.Fatal("instantiated pair was counted")
	}
	astTestMust(t, state.gc())
	if len(state.apps) != 1 || len(state.toInstantiate) != 0 || state.counts[second] != 2 || len(state.instantiated) != 1 {
		t.Fatal("GC must release instantiated pair and decay the survivor", state.apps, state.counts, state.instantiated)
	}
	astTestMust(t, state.clauseDeleted(first))
	astTestMust(t, state.congruenceEvent(apps[0], apps[1]))
	if len(state.apps) != 2 || state.counts[first] != 1 {
		t.Fatal("clause deletion must allow new pair observation", state.apps, state.counts)
	}
	astTestMust(t, state.markInstantiated(second))
	astTestMust(t, state.initSearch())
	if len(state.apps) != 0 || len(state.counts) != 0 || len(state.toInstantiate) != 0 || len(state.instantiated) != 1 {
		t.Fatal("init_search must retain instantiated clauses", state.apps, state.counts, state.instantiated)
	}
	astTestMust(t, state.reset())
	if len(state.instantiated) != 0 {
		t.Fatal("reset must discard instantiated clauses")
	}
	for i := 0; i < 3; i++ {
		astTestMust(t, state.congruenceEvent(apps[0], apps[1]))
	}
	astTestMust(t, state.markInstantiated(first))
	astTestMust(t, state.clauseDeleted(first))
	for i := 0; i < 5; i++ {
		astTestMust(t, state.congruenceEvent(apps[0], apps[1]))
	}
	astTestMust(t, state.gc())
	if len(state.apps) != 2 || state.counts[first] != 2 || len(state.toInstantiate) != 1 {
		t.Fatal("reobserved pair rows must decay in source row order", state.apps, state.counts, state.toInstantiate)
	}
	astTestMust(t, state.reset())
	for h, want := range base {
		refs, e := m.GetRefCount(h)
		astTestMust(t, e)
		if refs != want {
			t.Fatal("app reference leaked after pair GC/reset", h, refs, want)
		}
	}
}

// AUDIT.md smtcore_03-F7: dyn_ack_params copies smt.dack.gc_inv_decay
// unchecked, and gc's static_cast<unsigned>(num_occs * decay) is defined
// for a decay above 1.
func TestDynAckPairStateAcceptsDecayAboveOne(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkSort(symbol.Numerical(901), ast.ASTSortInfo{})
	astTestMust(t, e)
	decl, e := m.MkFuncDecl(symbol.Numerical(902), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	apps := make([]ast.ASTHandle, 2)
	for i := range apps {
		arg, e := m.MkConst(symbol.Numerical(uint32(903+i)), sort)
		astTestMust(t, e)
		apps[i], e = m.MkApp(decl, []ast.ASTHandle{arg})
		astTestMust(t, e)
		astTestMust(t, m.IncRef(apps[i]))
		h := apps[i]
		t.Cleanup(func() { astTestMust(t, m.DecRef(h)) })
	}
	state, e := newDynAckPairState(m, 10, 1.5)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, state.close()) })
	for i := 0; i < 3; i++ {
		astTestMust(t, state.congruenceEvent(apps[0], apps[1]))
	}
	astTestMust(t, state.gc())
	if count := state.counts[dynAckPair{apps[0], apps[1]}]; count != 4 { // static_cast<unsigned>(3 * 1.5)
		t.Fatal("gc did not scale the occurrence count by the decay", count)
	}
}
