package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtCGTestDeclaration(t testing.TB, manager *ast.ASTManager, name uint32, sort ast.ASTHandle, flags ast.ASTFuncFlags) ast.ASTHandle {
	t.Helper()
	info, err := manager.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, flags, false)
	astTestMust(t, err)
	declaration, err := manager.MkFuncDeclWithInfo(symbol.Numerical(name), []ast.ASTHandle{sort, sort}, sort, info)
	astTestMust(t, err)
	astTestMust(t, manager.DelFuncDeclInfo(&info))
	return declaration
}

func smtCGTestApplication(t testing.TB, manager *ast.ASTManager, declaration ast.ASTHandle, handles []ast.ASTHandle, nodes []*SMTEnode) *SMTEnode {
	t.Helper()
	owner, err := manager.MkApp(declaration, handles)
	astTestMust(t, err)
	byApp := make(map[ast.ASTHandle]*SMTEnode, len(handles))
	for i := range handles {
		byApp[handles[i]] = nodes[i]
	}
	node, err := NewSMTEnode(manager, owner, smtEnodeMapLookup(byApp), SMTEnodeOptions{CongruenceEnabled: true})
	astTestMust(t, err)
	return node
}

func TestSMTCongruenceTableRegistersDeclarationsOnLookupMiss(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(symbol.Numerical(2590), ast.ASTSortInfo{})
	astTestMust(t, err)
	a, an := smtEnodeConstant(t, m, 2591, sort, 0)
	args, nodes := []ast.ASTHandle{a, a}, []*SMTEnode{an, an}
	f := smtCGTestDeclaration(t, m, 2592, sort, ast.ASTFuncFlags{})
	g := smtCGTestDeclaration(t, m, 2593, sort, ast.ASTFuncFlags{})
	faa := smtCGTestApplication(t, m, f, args, nodes)
	gaa := smtCGTestApplication(t, m, g, args, nodes)
	refs := func(h ast.ASTHandle) uint32 {
		n, err := m.GetRefCount(h)
		astTestMust(t, err)
		return n
	}
	beforeF, beforeG := refs(f), refs(g)
	table, err := NewSMTCongruenceTable(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, table.Close()) }()

	if found, err := table.FindApp(g, nodes, false); err != nil || found != nil {
		t.Fatalf("find g(a,a) in empty table: %p, %v", found, err)
	}
	if found, _, err := table.Find(faa); err != nil || found != nil {
		t.Fatalf("find f(a,a) in empty table: %p, %v", found, err)
	}
	if refs(g) != beforeG+1 || refs(f) != beforeF+1 {
		t.Fatalf("lookup references on g, f: got %d, %d, want %d, %d", refs(g), refs(f), beforeG+1, beforeF+1)
	}
	// Insert in reverse order. The first lookups already assigned the IDs.
	_, err = table.Insert(faa)
	astTestMust(t, err)
	_, err = table.Insert(gaa)
	astTestMust(t, err)
	if gaa.FunctionDeclID() != 0 || faa.FunctionDeclID() != 1 {
		t.Fatalf("IDs in first lookup order: g=%d f=%d", gaa.FunctionDeclID(), faa.FunctionDeclID())
	}
	astTestMust(t, table.Reset())
	if refs(f) != beforeF || refs(g) != beforeG {
		t.Fatalf("references after reset: f=%d g=%d, want %d, %d", refs(f), refs(g), beforeF, beforeG)
	}
	// The same live declaration must register again after reset. The old
	// per-declaration bucket still contained the previous inserted node.
	if found, err := table.FindApp(g, nodes, false); err != nil || found != nil || refs(g) != beforeG+1 {
		t.Fatal("lookup reused a bucket from before reset", found, err, refs(g))
	}
	astTestMust(t, table.Reset())

}

func TestSMTCongruenceTableExactCommutativeInsertFindEraseReset(t *testing.T) {
	manager := astTestManager(t)
	sort, err := manager.MkSort(symbol.Numerical(2600), ast.ASTSortInfo{})
	astTestMust(t, err)
	a, an := smtEnodeConstant(t, manager, 2601, sort, 0)
	b, bn := smtEnodeConstant(t, manager, 2602, sort, 0)
	declaration := smtCGTestDeclaration(t, manager, 2603, sort, ast.ASTFuncFlags{Commutative: true})
	ab := smtCGTestApplication(t, manager, declaration, []ast.ASTHandle{a, b}, []*SMTEnode{an, bn})
	ba := smtCGTestApplication(t, manager, declaration, []ast.ASTHandle{b, a}, []*SMTEnode{bn, an})
	table, err := NewSMTCongruenceTable(manager)
	astTestMust(t, err)
	inserted, err := table.Insert(ab)
	astTestMust(t, err)
	if inserted.Node != ab || inserted.Commuted {
		t.Fatal("first insertion", inserted)
	}
	inserted, err = table.Insert(ba)
	astTestMust(t, err)
	if inserted.Node != ab || !inserted.Commuted {
		t.Fatal("commutative match", inserted)
	}
	if found, commuted, err := table.Find(ba); err != nil || found != ab || !commuted {
		t.Fatal("find", found, commuted, err)
	}
	if contains, err := table.ContainsPointer(ba); err != nil || contains {
		t.Fatal("nonrepresentative pointer", contains, err)
	}
	if err := table.CheckInvariant(); err != nil {
		t.Fatal(err)
	}
	if erased, err := table.Erase(ba); err != nil || !erased {
		t.Fatal("erase congruent key", erased, err)
	}
	if found, _, err := table.Find(ab); err != nil || found != nil {
		t.Fatal("find after erase", found, err)
	}
	astTestMust(t, table.Reset())
	astTestMust(t, table.Close())
}

// smtcore_02-F2: cg_table::mk_table_for gives a flat-associative binary
// declaration (e.g. +) the positional n-ary table, not comm_table, even
// when it is commutative.
func TestSMTCongruenceTableFlatAssociativeBinaryIsPositional(t *testing.T) {
	manager := astTestManager(t)
	sort, err := manager.MkSort(symbol.Numerical(2650), ast.ASTSortInfo{})
	astTestMust(t, err)
	a, an := smtEnodeConstant(t, manager, 2651, sort, 0)
	b, bn := smtEnodeConstant(t, manager, 2652, sort, 0)
	declaration := smtCGTestDeclaration(t, manager, 2653, sort, ast.ASTFuncFlags{Commutative: true, FlatAssociative: true})
	ab := smtCGTestApplication(t, manager, declaration, []ast.ASTHandle{a, b}, []*SMTEnode{an, bn})
	ba := smtCGTestApplication(t, manager, declaration, []ast.ASTHandle{b, a}, []*SMTEnode{bn, an})
	table, err := NewSMTCongruenceTable(manager)
	astTestMust(t, err)
	inserted, err := table.Insert(ab)
	astTestMust(t, err)
	if inserted.Node != ab || inserted.Commuted {
		t.Fatal("first insertion", inserted)
	}
	if found, commuted, err := table.Find(ba); err != nil || found != nil || commuted {
		t.Fatal("flat-associative find matched modulo commutativity", found, commuted, err)
	}
	inserted, err = table.Insert(ba)
	astTestMust(t, err)
	if inserted.Node != ba || inserted.Commuted {
		t.Fatal("flat-associative insertion matched modulo commutativity", inserted)
	}
	if err := table.CheckInvariant(); err != nil {
		t.Fatal(err)
	}
	astTestMust(t, table.Reset())
	astTestMust(t, table.Close())
}

func TestSMTCongruenceTableNativeSignaturesAndNaryCollisions(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(symbol.Numerical(2700), ast.ASTSortInfo{})
	astTestMust(t, err)
	var terms []ast.ASTHandle
	var nodes []*SMTEnode
	for i := 0; i < 4; i++ {
		h, n := smtEnodeConstant(t, m, uint32(2701+i), sort, 0)
		terms = append(terms, h)
		nodes = append(nodes, n)
	}
	declaration := smtCGTestDeclaration(t, m, 2710, sort, ast.ASTFuncFlags{Commutative: true})
	table, err := NewSMTCongruenceTable(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, table.Close()) }()
	var representatives [4][4]*SMTEnode
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			n := smtCGTestApplication(t, m, declaration, []ast.ASTHandle{terms[i], terms[j]}, []*SMTEnode{nodes[i], nodes[j]})
			inserted, err := table.Insert(n)
			astTestMust(t, err)
			if i <= j {
				representatives[i][j] = n
				if inserted.Node != n || inserted.Commuted {
					t.Fatal("first unordered pair", i, j)
				}
			} else if inserted.Node != representatives[j][i] || !inserted.Commuted {
				t.Fatal("reverse unordered pair", i, j)
			}
			if n.FunctionDeclID() != 0 {
				t.Fatal("declaration id must be assigned to every inserted node", n.FunctionDeclID())
			}
		}
	}
	astTestMust(t, table.CheckInvariant())
	astTestMust(t, table.Reset())
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	f, err := m.MkFuncDeclWithInfo(symbol.Numerical(2711), []ast.ASTHandle{sort, sort, sort}, sort, info)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	var triples [4][4][4]*SMTEnode
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 4; k++ {
				n := smtCGTestApplication(t, m, f, []ast.ASTHandle{terms[i], terms[j], terms[k]}, []*SMTEnode{nodes[i], nodes[j], nodes[k]})
				inserted, err := table.Insert(n)
				astTestMust(t, err)
				if inserted.Node != n || inserted.Commuted {
					t.Fatal("distinct ordered triple collapsed", i, j, k)
				}
				triples[i][j][k] = n
			}
		}
	}
	// A separate root with the same AST owner deliberately collides in the
	// n-ary suffix key. The exact root comparison must keep it independent.
	alias, err := NewSMTEnode(m, terms[2], nil, SMTEnodeOptions{})
	astTestMust(t, err)
	collision := smtCGTestApplication(t, m, f, []ast.ASTHandle{terms[0], terms[1], terms[2]}, []*SMTEnode{nodes[0], nodes[1], alias})
	inserted, err := table.Insert(collision)
	astTestMust(t, err)
	if inserted.Node != collision {
		t.Fatal("owner identity collision changed congruence")
	}
	if found, _, err := table.Find(triples[0][1][2]); err != nil || found != triples[0][1][2] {
		t.Fatal(found, err)
	}
	if erased, err := table.Erase(collision); err != nil || !erased {
		t.Fatal(erased, err)
	}
	if found, _, err := table.Find(triples[0][1][2]); err != nil || found != triples[0][1][2] {
		t.Fatal("collision erase removed a different class", found, err)
	}
	astTestMust(t, table.CheckInvariant())
}

func TestSMTAlmostCongruenceTableGroupsSpecialRoots(t *testing.T) {
	manager := astTestManager(t)
	sort, err := manager.MkSort(symbol.Numerical(2610), ast.ASTSortInfo{})
	astTestMust(t, err)
	a, an := smtEnodeConstant(t, manager, 2611, sort, 0)
	b, bn := smtEnodeConstant(t, manager, 2612, sort, 0)
	c, cn := smtEnodeConstant(t, manager, 2613, sort, 0)
	declaration := smtCGTestDeclaration(t, manager, 2614, sort, ast.ASTFuncFlags{})
	ac := smtCGTestApplication(t, manager, declaration, []ast.ASTHandle{a, c}, []*SMTEnode{an, cn})
	bc := smtCGTestApplication(t, manager, declaration, []ast.ASTHandle{b, c}, []*SMTEnode{bn, cn})
	ca := smtCGTestApplication(t, manager, declaration, []ast.ASTHandle{c, a}, []*SMTEnode{cn, an})
	table := NewSMTAlmostCongruenceTable(an, bn)
	for _, node := range []*SMTEnode{ac, bc, ca} {
		astTestMust(t, table.Insert(node))
	}
	group, err := table.Find(ac)
	astTestMust(t, err)
	if len(group) != 2 || group[0] != bc || group[1] != ac {
		t.Fatal("source prepend group order", group)
	}
	other, err := table.Find(ca)
	astTestMust(t, err)
	if len(other) != 1 || other[0] != ca {
		t.Fatal("independent group", other)
	}
	if erased, err := table.Erase(bc); err != nil || !erased {
		t.Fatal("erase equivalence class", erased, err)
	}
	if group, err = table.Find(ac); err != nil || group != nil {
		t.Fatal("erased group remained", group, err)
	}
	astTestMust(t, table.ResetFor(an, cn))
	if !table.Empty() {
		t.Fatal("reset for roots")
	}
}

// Commutative binary and unary entries must round-trip through erase, and an
// erased signature must accept a new representative.
func TestSMTCongruenceTableEraseAndReinsert(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(symbol.Numerical(2800), ast.ASTSortInfo{})
	astTestMust(t, err)
	a, an := smtEnodeConstant(t, m, 2801, sort, 0)
	b, bn := smtEnodeConstant(t, m, 2802, sort, 0)
	plus := smtCGTestDeclaration(t, m, 2803, sort, ast.ASTFuncFlags{Commutative: true})
	table, err := NewSMTCongruenceTable(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, table.Close()) }()
	first := smtCGTestApplication(t, m, plus, []ast.ASTHandle{a, b}, []*SMTEnode{an, bn})
	second := smtCGTestApplication(t, m, plus, []ast.ASTHandle{b, a}, []*SMTEnode{bn, an})
	inserted, err := table.Insert(first)
	astTestMust(t, err)
	if inserted.Node != first {
		t.Fatal("first insert")
	}
	inserted, err = table.Insert(second)
	astTestMust(t, err)
	if inserted.Node != first || !inserted.Commuted {
		t.Fatal("commuted lookup")
	}
	erased, err := table.Erase(second)
	astTestMust(t, err)
	if !erased {
		t.Fatal("erasing through a commuted congruent node must remove its representative")
	}
	if found, _, err := table.Find(first); err != nil || found != nil {
		t.Fatal("representative survived erase", found, err)
	}
	erased, err = table.Erase(first)
	astTestMust(t, err)
	if erased {
		t.Fatal("double erase reported success")
	}
	inserted, err = table.Insert(second)
	astTestMust(t, err)
	if inserted.Node != second || inserted.Commuted {
		t.Fatal("erased signature did not accept a new representative")
	}
	astTestMust(t, table.CheckInvariant())
}

func TestSMTCongruenceEraseChecksDistinctNodeAtStaleKey(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(symbol.Numerical(2820), ast.ASTSortInfo{})
	astTestMust(t, err)
	a, an := smtEnodeConstant(t, m, 2821, sort, 0)
	b, bn := smtEnodeConstant(t, m, 2822, sort, 0)
	c, cn := smtEnodeConstant(t, m, 2823, sort, 0)
	f := smtCGTestDeclaration(t, m, 2824, sort, ast.ASTFuncFlags{})
	query := smtCGTestApplication(t, m, f, []ast.ASTHandle{a, b}, []*SMTEnode{an, bn})
	stored := smtCGTestApplication(t, m, f, []ast.ASTHandle{c, b}, []*SMTEnode{cn, bn})
	table, err := NewSMTCongruenceTable(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, table.Close()) }()
	cn.root = an
	_, err = table.Insert(stored)
	astTestMust(t, err)

	// After splitting c from a, the captured key is still (a,b),
	// but the stored application now represents (c,b).
	cn.root = cn
	erased, err := table.Erase(query)
	cn.root = an
	if err != nil || erased {
		t.Fatal("erased noncongruent representative at a stale key", erased, err)
	}
	if erased, err := table.Erase(stored); err != nil || !erased {
		t.Fatal("lost original representative", erased, err)
	}
}

// Merging must not allocate per-lookup signature keys for arity two terms.
func TestSMTCongruenceTableBinaryOperationsDoNotAllocate(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(symbol.Numerical(2810), ast.ASTSortInfo{})
	astTestMust(t, err)
	a, an := smtEnodeConstant(t, m, 2811, sort, 0)
	b, bn := smtEnodeConstant(t, m, 2812, sort, 0)
	declaration := smtCGTestDeclaration(t, m, 2813, sort, ast.ASTFuncFlags{})
	table, err := NewSMTCongruenceTable(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, table.Close()) }()
	node := smtCGTestApplication(t, m, declaration, []ast.ASTHandle{a, b}, []*SMTEnode{an, bn})
	_, err = table.Insert(node)
	astTestMust(t, err)
	if allocs := testing.AllocsPerRun(200, func() {
		_, _ = table.Erase(node)
		_, _ = table.Insert(node)
		_, _, _ = table.Find(node)
	}); allocs != 0 {
		t.Fatalf("%v allocations per erase/insert/find", allocs)
	}
}

func TestSMTCongruenceParentEraseValidatesSlotAndCurrentRoots(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(symbol.Numerical(2900), ast.ASTSortInfo{})
	astTestMust(t, err)
	a, an := smtEnodeConstant(t, m, 2901, sort, 0)
	b, bn := smtEnodeConstant(t, m, 2902, sort, 0)
	f := smtCGTestDeclaration(t, m, 2903, sort, ast.ASTFuncFlags{})
	ab := smtCGTestApplication(t, m, f, []ast.ASTHandle{a, b}, []*SMTEnode{an, bn})
	ba := smtCGTestApplication(t, m, f, []ast.ASTHandle{b, a}, []*SMTEnode{bn, an})
	table, err := NewSMTCongruenceTable(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, table.Close()) }()
	for _, n := range []*SMTEnode{ab, ba} {
		_, err = table.Insert(n)
		astTestMust(t, err)
	}

	// The entry still points at ab, but its captured roots no longer match.
	an.root = bn
	erased, err := table.eraseParent(ab)
	an.root = an
	if err != nil || erased {
		t.Fatalf("erased changed key: %v, %v", erased, err)
	}

	// A second table may have left a hint for an unrelated occupied slot.
	ab.congruenceSlot = ba.congruenceSlot
	if erased, err = table.eraseParent(ab); err != nil || !erased {
		t.Fatalf("stale slot: %v, %v", erased, err)
	}
	if node, _, err := table.Find(ba); err != nil || node != ba {
		t.Fatalf("lost other representative: %p, %v", node, err)
	}
	if erased, err = table.eraseParent(ab); err != nil || erased {
		t.Fatalf("double erase: %v, %v", erased, err)
	}
	astTestMust(t, table.CheckInvariant())
}
