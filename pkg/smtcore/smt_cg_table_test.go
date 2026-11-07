package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtCGTestDeclaration(t *testing.T, manager *ast.ASTManager, name uint32, sort ast.ASTHandle, flags ast.ASTFuncFlags) ast.ASTHandle {
	t.Helper()
	info, err := manager.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, flags, false)
	astTestMust(t, err)
	declaration, err := manager.MkFuncDeclWithInfo(symbol.Numerical(name), []ast.ASTHandle{sort, sort}, sort, info)
	astTestMust(t, err)
	astTestMust(t, manager.DelFuncDeclInfo(&info))
	return declaration
}

func smtCGTestApplication(t *testing.T, manager *ast.ASTManager, declaration ast.ASTHandle, handles []ast.ASTHandle, nodes []*SMTEnode) *SMTEnode {
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
