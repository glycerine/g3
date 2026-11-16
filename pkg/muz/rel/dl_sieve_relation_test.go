package rel

// Source: src/muz/rel/dl_sieve_relation.cpp (sieve_relation_plugin).

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func sieveTestManager(t *testing.T) (*RelationManager, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	manager := astTestManager(t)
	m, err := NewRelationManager(manager)
	astTestMust(t, err)
	t.Cleanup(func() { _ = m.Close() })
	astTestMust(t, m.RegisterRelationPlugin(NewIntervalRelationPlugin(m)))
	arithmetic, err := manager.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	return m, integer, boolean
}

// TestSieveRelationColumnsAndKinds checks full(p, s, inner_plugin):
// extract_inner_columns keeps the columns the inner plugin handles, a sieve
// kind of the same spec is shared, and can_handle_signature(s, kind) holds
// only for the signature the kind was made for.
func TestSieveRelationColumnsAndKinds(t *testing.T) {
	m, integer, boolean := sieveTestManager(t)
	sorts := []ast.ASTHandle{integer, boolean}
	table := m.tableRelationPluginNamed("hashtable")
	sieve := m.sievePlugin()
	if ok, err := sieve.CanHandleSignature(sorts); err != nil || ok {
		t.Fatal("sieve handles a signature by default", ok, err)
	}
	full, err := sieve.full(ast.ASTHandle{}, sorts, table)
	astTestMust(t, err)
	defer full.Close()
	if !boolsEqual(full.innerCols, []bool{false, true}) || len(full.inner.RelationSignature()) != 1 {
		t.Fatal("inner columns", full.innerCols)
	}
	if empty, err := full.Empty(); err != nil || empty {
		t.Fatal("full sieve is empty", empty, err)
	}
	again, err := sieve.full(ast.ASTHandle{}, sorts, table)
	astTestMust(t, err)
	defer again.Close()
	if again.kind != full.kind {
		t.Fatal("same spec, different kinds", again.kind, full.kind)
	}
	if ok, _ := sieve.CanHandleSignatureKind(sorts, full.kind); !ok {
		t.Fatal("sieve kind refused for its signature")
	}
	if ok, _ := sieve.CanHandleSignatureKind([]ast.ASTHandle{boolean}, full.kind); ok {
		t.Fatal("sieve kind accepted for another signature")
	}
	empty, err := m.mkEmptyRelationKind(sorts, full.kind)
	astTestMust(t, err)
	defer empty.Close()
	se, ok := isSieveRelation(empty)
	if !ok || se.kind != full.kind {
		t.Fatal("mk_empty_relation of a sieve kind", empty.RelationPlugin().Name())
	}
	if e, err := se.Empty(); err != nil || !e {
		t.Fatal("empty sieve not empty", e, err)
	}
	var out bytes.Buffer
	astTestMust(t, full.Display(&out))
	if got := out.String(); len(got) < 21 || got[:21] != "Sieve relation (0,1)\n" {
		t.Fatalf("display %q", got)
	}
}

// TestSieveRelationOperations runs project, rename, filter_equal and union
// through the relation manager on sieves over a hashtable inner relation.
func TestSieveRelationOperations(t *testing.T) {
	m, integer, boolean := sieveTestManager(t)
	sorts := []ast.ASTHandle{integer, boolean}
	table := m.tableRelationPluginNamed("hashtable")
	sieve := m.sievePlugin()
	full, err := sieve.full(ast.ASTHandle{}, sorts, table)
	astTestMust(t, err)
	defer full.Close()

	// rename (0 1): the Bool column moves first.
	rename, err := m.MkRelationRenameFn(full, []int{0, 1})
	astTestMust(t, err)
	renamed, err := rename(full)
	astTestMust(t, err)
	defer renamed.Close()
	sr := renamed.(*sieveRelation)
	if !boolsEqual(sr.innerCols, []bool{true, false}) || sr.sorts[0] != boolean || sr.sorts[1] != integer {
		t.Fatal("rename", sr.innerCols)
	}

	// filter_equal on the inner column keeps one row; on the sieved
	// column it is the identity.
	value, err := m.manager.MkTrue()
	astTestMust(t, err)
	filter, err := m.MkRelationFilterEqualFn(sr, value, 0)
	astTestMust(t, err)
	astTestMust(t, filter(sr))
	if n, err := sr.inner.Len(); err != nil || n != 1 {
		t.Fatal("filter_equal rows", n, err)
	}

	// project the Int column: no inner column is removed.
	project, err := m.MkRelationProjectFn(sr, []int{1})
	astTestMust(t, err)
	projected, err := project(sr)
	astTestMust(t, err)
	defer projected.Close()
	ps := projected.(*sieveRelation)
	if !ps.noSievedColumns() || len(ps.sorts) != 1 {
		t.Fatal("project", ps.innerCols)
	}

	// union of two aligned sieves.
	target, err := mkEmptyLike(full)
	astTestMust(t, err)
	defer target.Close()
	union, err := m.MkRelationUnionFn(target, full, nil)
	astTestMust(t, err)
	if union == nil {
		t.Fatal("no union of aligned sieves")
	}
	astTestMust(t, union(target, full, nil))
	if n, err := target.Len(); err != nil || n != 2 {
		t.Fatal("union rows", n, err)
	}
	// a sieve with sieved columns does not union into an unsieved
	// relation of another kind.
	inner, err := table.MkEmpty(sorts[1:])
	astTestMust(t, err)
	defer inner.Close()
	if fn, err := sieve.MkUnionFn(inner, renamed, nil); err != nil || fn != nil {
		t.Fatal("misaligned union", fn != nil, err)
	}
}
