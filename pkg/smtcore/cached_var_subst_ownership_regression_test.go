package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// C++ cached_var_subst::m_refs stores owning expression pointers, not one
// separately allocated API-reference slot for every quantifier/binding/result.
// A million six-variable entries must not create eight million ASTRef slots.
func TestSMTCachedVarSubstitutionDoesNotAllocateAPIReferencePerCachedHandle(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "cached-root-x")}, Body: variable})
	astTestMust(t, err)
	astTestMust(t, m.IncRef(quantifier))
	t.Cleanup(func() { astTestMust(t, m.DecRef(quantifier)) })
	cache, err := NewSMTCachedVarSubstitution(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cache.Close()) })
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&out)) })
	var warmReferences uint32
	for i := 0; i < 256; i++ {
		owner, binding := smtEnodeConstant(t, m, uint32(34000+i), boolean, 0)
		astTestMust(t, m.IncRef(owner))
		h := owner
		t.Cleanup(func() { astTestMust(t, m.DecRef(h)) })
		cached, e := cache.Instantiate(quantifier, []*SMTEnode{binding}, out)
		astTestMust(t, e)
		if cached {
			t.Fatal("new binding reported a cache hit")
		}
		value, e := m.RefGet(out)
		astTestMust(t, e)
		if value != owner {
			t.Fatal("wrong substituted result")
		}
		if i == 0 {
			warmReferences = m.Stats().LiveReferences
		}
	}
	if refs := m.Stats().LiveReferences; refs != warmReferences {
		t.Fatalf("cache allocated %d extra API reference slots after warmup; want none", refs-warmReferences)
	}
	entryCapacity, refCapacity := cap(cache.list), cap(cache.refs)
	astTestMust(t, cache.Reset())
	if count, e := m.GetRefCount(quantifier); e != nil || count != 1 {
		t.Fatal("reset did not release cached quantifier ownership", count, e)
	}
	if cache.Size() != 0 {
		t.Fatal("reset retained cached substitutions")
	}
	if cap(cache.list) != entryCapacity || cap(cache.refs) != refCapacity || len(cache.entries) != 0 {
		t.Fatal("busy batch lost reusable storage or retained a key")
	}
	astTestMust(t, cache.Reset())
	if cache.entries != nil || cache.list != nil || cache.refs != nil {
		t.Fatal("empty batch retained the previous peak")
	}
}

// The cache must remain the AST owner after caller roots and substitution
// scratch state disappear, including when binding and result are the same AST.
func TestSMTCachedVarSubstitutionOwnsBindingsUntilReset(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(35000)}, Body: variable})
	astTestMust(t, err)
	astTestMust(t, m.IncRef(quantifier))
	owner, binding := smtEnodeConstant(t, m, 35001, boolean, 0)
	astTestMust(t, m.IncRef(owner))
	cache, err := NewSMTCachedVarSubstitution(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cache.Close()) })
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&out)) })
	_, err = cache.Instantiate(quantifier, []*SMTEnode{binding}, out)
	astTestMust(t, err)
	astTestMust(t, m.SetRef(out, ast.ASTHandle{}))
	astTestMust(t, cache.substitution.Reset())
	astTestMust(t, m.DecRef(quantifier))
	astTestMust(t, m.DecRef(owner))
	if !m.IsLiveHandle(quantifier) || !m.IsLiveHandle(owner) {
		t.Fatal("cache failed to own its input/output ASTs")
	}
	cached, err := cache.Instantiate(quantifier, []*SMTEnode{binding}, out)
	astTestMust(t, err)
	if !cached {
		t.Fatal("retained binding missed the cache")
	}
	astTestMust(t, m.SetRef(out, ast.ASTHandle{}))
	astTestMust(t, cache.Reset())
	astTestMust(t, cache.Reset())
	astTestMust(t, cache.Close())
	if cache.Size() != 0 || m.IsLiveHandle(quantifier) || m.IsLiveHandle(owner) {
		t.Fatal("reset retained cached ownership")
	}
}
