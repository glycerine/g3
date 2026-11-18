package model

// Native collection checks for src/model/model_evaluator.cpp:337-416.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/hash"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func arrayTableMust(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
func arrayTableManager(t *testing.T) *ast.ASTManager {
	b := arena.NewBudget(1 << 30)
	s, e := usymbol.NewManager(usymbol.Config{Budget: b})
	arrayTableMust(t, e)
	q, e := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	arrayTableMust(t, e)
	t.Cleanup(func() {
		arrayTableMust(t, q.Close())
		arrayTableMust(t, s.Close())
		if b.Used() != 0 {
			t.Fatal("tuple fixture leak", b.Used())
		}
	})
	m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: s, Numerals: q, Budget: b})
	arrayTableMust(t, e)
	t.Cleanup(func() { arrayTableMust(t, m.Close()) })
	return m
}
func arrayTableRow(t *testing.T, m *ast.ASTManager, n, key uint32, value bool) []ast.ASTHandle {
	t.Helper()
	var row []ast.ASTHandle
	t.Cleanup(func() { arrayTableMust(t, releaseArrayRoots(m, &row)) })
	for i := uint32(0); i < n; i++ {
		h, e := m.MkBoolVal(key&(1<<i) != 0)
		arrayTableMust(t, e)
		arrayTableMust(t, appendArrayRoot(m, &row, h))
	}
	h, e := m.MkBoolVal(value)
	arrayTableMust(t, e)
	arrayTableMust(t, appendArrayRoot(m, &row, h))
	return row
}

func TestArrayTupleNativeReplacementAndRemoval(t *testing.T) {
	m := arrayTableManager(t)
	table := arrayTupleTable{manager: m, arity: 8}
	defer table.close()
	rows := make([][]ast.ASTHandle, 256)
	for i := range rows {
		rows[i] = arrayTableRow(t, m, 8, uint32(i), true)
	}
	replacement := arrayTableRow(t, m, 8, 0, false)
	before := m.Budget().Used()
	m.Budget().FailAfter(0)
	defer m.Budget().FailAfter(-1)
	for _, row := range rows {
		arrayTableMust(t, table.insert(row))
	}
	arrayTableMust(t, table.insert(replacement))
	found, ok, e := table.find(rows[0])
	arrayTableMust(t, e)
	if !ok || found[8] != replacement[8] || table.size != 256 {
		t.Fatal("equal index tuple must replace its value", ok, len(found), table.size)
	}
	// Every live row is visited once, in table order.
	checkLive := func(want int) {
		t.Helper()
		got := table.entries()
		if len(got) != want {
			t.Fatal("iteration count", len(got), want)
		}
		for _, row := range got {
			other, ok, e := table.find(row)
			arrayTableMust(t, e)
			if !ok || other[8] != row[8] {
				t.Fatal("iterated row is not live")
			}
		}
	}
	checkLive(256)
	arrayTableMust(t, table.remove(rows[0]))
	if _, ok, e := table.find(replacement); e != nil || ok {
		t.Fatal("removed tuple", ok, e)
	}
	checkLive(255)
	arrayTableMust(t, table.insert(rows[0]))
	checkLive(256)
	if m.Budget().Used() != before {
		t.Fatal("native table charged arena backing")
	}
}

// AUDIT.md model_01-F3: mk_array_eq_core walks table1 in core_hashtable
// order (capacity 8, linear probing on args_hash = get_composite_hash with
// kind hash 17), not insertion order. The end-to-end order is checked by
// the eval_array_eq_table_order.smt2 vector; here the slots are recomputed
// independently from the AST hashes.
func TestArrayTupleTableHashtableOrder(t *testing.T) {
	m := arrayTableManager(t)
	_, e := m.RegisterArithmeticPlugin()
	arrayTableMust(t, e)
	var keep []ast.ASTHandle
	t.Cleanup(func() { arrayTableMust(t, releaseArrayRoots(m, &keep)) })
	table := arrayTupleTable{manager: m, arity: 1}
	defer table.close()
	var slots [8]ast.ASTHandle
	var used [8]bool
	for _, k := range []int64{9, 42, 300, 5, 17} {
		h, e := m.ArithmeticNumeralInt64(int64(k), true)
		arrayTableMust(t, e)
		arrayTableMust(t, appendArrayRoot(m, &keep, h))
		arrayTableMust(t, table.insert([]ast.ASTHandle{h, h}))
		child, e := m.GetHash(h)
		arrayTableMust(t, e)
		idx := hash.Composite(17, 1, func(uint32) uint32 { return child }) & 7
		for used[idx] {
			idx = (idx + 1) & 7
		}
		slots[idx], used[idx] = h, true
	}
	var want []ast.ASTHandle
	for i, h := range slots {
		if used[i] {
			want = append(want, h)
		}
	}
	got := table.entries()
	if len(got) != len(want) {
		t.Fatal("hashtable size", len(got))
	}
	for i := range want {
		if got[i][0] != want[i] {
			t.Fatal("hashtable order differs at", i)
		}
	}
}

func TestArrayTupleNativeCompleteIndexIdentityAndBorrowing(t *testing.T) {
	m := arrayTableManager(t)
	a := arrayTableRow(t, m, 2, 1, true)
	b := arrayTableRow(t, m, 2, 2, true)
	table := arrayTupleTable{manager: m, arity: 2}
	before, e := m.GetRefCount(a[0])
	arrayTableMust(t, e)
	arrayTableMust(t, table.insert(a))
	arrayTableMust(t, table.insert(b))
	if table.size != 2 {
		t.Fatal("ordered tuple elements collapsed")
	}
	for _, row := range [][]ast.ASTHandle{a, b} {
		got, ok, e := table.find(row)
		arrayTableMust(t, e)
		if !ok || got[0] != row[0] || got[1] != row[1] {
			t.Fatal("tuple lookup", got, ok)
		}
	}
	if e := table.insert(a[:2]); e != ast.ErrASTDomain {
		t.Fatal("missing tuple result", e)
	}
	table.close()
	after, e := m.GetRefCount(a[0])
	arrayTableMust(t, e)
	if before != after {
		t.Fatal("borrowed table changed AST ownership", before, after)
	}
}

func TestModelArrayNativeRootsPartialAppendAndGrowth(t *testing.T) {
	m := arrayTableManager(t)
	yes, e := m.MkTrue()
	arrayTableMust(t, e)
	before, e := m.GetRefCount(yes)
	arrayTableMust(t, e)
	var roots []ast.ASTHandle
	foreign := arrayTableManager(t)
	no, e := foreign.MkFalse()
	arrayTableMust(t, e)
	e = appendArrayArguments(m, &roots, ast.SliceArguments([]ast.ASTHandle{yes, no}))
	if e != ast.ErrASTHandle || len(roots) != 1 {
		t.Fatal("partial owning append", len(roots), e)
	}
	m.Budget().FailAfter(0)
	for i := 0; i < 4096; i++ {
		arrayTableMust(t, appendArrayRoot(m, &roots, yes))
	}
	count, e := m.GetRefCount(yes)
	arrayTableMust(t, e)
	if count != before+4097 {
		t.Fatal("one reference per native root", count, before)
	}
	arrayTableMust(t, releaseArrayRoots(m, &roots))
	m.Budget().FailAfter(-1)
	count, e = m.GetRefCount(yes)
	arrayTableMust(t, e)
	if len(roots) != 0 || count != before {
		t.Fatal("native root release", len(roots), count, before)
	}
}
