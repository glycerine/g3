package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Raft's log_matching multi-pattern repeatedly continues joins against a large
// declaration bucket. C++ interpreter::init_continue (src/smt/mam.cpp) checks
// parentless joint roots before it enumerates that bucket. A rejected join
// must not allocate/copy the bucket, regardless of its size.
func TestSMTMAMContinueRejectsParentlessJointWithoutAllocating(t *testing.T) {
	m, state, _, _ := smtMAMFixture(t)
	sort, err := m.MkSort(astTestSymbol(t, m, "continue-sort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate := smtQuickPredicate(t, m, "continue-p", sort, boolean)
	for i := 0; i < 512; i++ {
		owner, node := smtEnodeConstant(t, m, uint32(30000+i), sort, 0)
		astTestMust(t, state.context.RegisterEnode(node, true))
		application, e := m.MkApp(predicate, []ast.ASTHandle{owner})
		astTestMust(t, e)
		parent, e := NewSMTEnode(m, application, state.context.enodes.get, SMTEnodeOptions{CongruenceEnabled: true})
		astTestMust(t, e)
		astTestMust(t, state.context.RegisterEnode(parent, true))
	}
	_, orphan := smtEnodeConstant(t, m, 31000, sort, 0)
	interpreter := newSMTMAMInterpreter(state, nil, false)
	interpreter.registers = []*SMTEnode{orphan}
	instruction := &smtMAMInstr{label: predicate, numArgs: 1, joints: []smtMAMJoint{{tag: mamVarTag, reg: 0}}}
	if got := interpreter.initContinue(instruction, 1); got != nil {
		t.Fatal("a parentless joint produced a match")
	}
	allocations := testing.AllocsPerRun(20, func() {
		if interpreter.initContinue(instruction, 1) != nil {
			panic("parentless joint matched")
		}
	})
	if allocations != 0 {
		t.Fatalf("rejected join copied the declaration bucket: %.0f allocations per rejection; want 0", allocations)
	}
}

// C++ keeps the filtered decl2enodes vector as context state; opening an
// iterator is O(1), rather than rebuilding a vector on every continuation.
func TestSMTMAMDeclarationEnumerationDoesNotCopy(t *testing.T) {
	m, state, _, _ := smtMAMFixture(t)
	sort, err := m.MkSort(astTestSymbol(t, m, "enumeration-sort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate := smtQuickPredicate(t, m, "enumeration-p", sort, boolean)
	owner, node := smtEnodeConstant(t, m, 32000, sort, 0)
	astTestMust(t, state.context.RegisterEnode(node, true))
	app, err := m.MkApp(predicate, []ast.ASTHandle{owner})
	astTestMust(t, err)
	parent, err := NewSMTEnode(m, app, state.context.enodes.get, SMTEnodeOptions{CongruenceEnabled: true})
	astTestMust(t, err)
	astTestMust(t, state.context.RegisterEnode(parent, true))
	it := newSMTMAMInterpreter(state, nil, false)
	if got := it.enodesOf(predicate); len(got) != 1 || got[0] != parent {
		t.Fatal("wrong declaration enumeration", got)
	}
	var got []*SMTEnode
	allocations := testing.AllocsPerRun(20, func() { got = it.enodesOf(predicate) })
	if len(got) != 1 || allocations != 0 {
		t.Fatalf("declaration enumeration copied its bucket: %.0f allocations; want 0", allocations)
	}
	if got := it.enodesOf(node.declaration); len(got) != 0 {
		t.Fatal("constants entered the matching index", got)
	}
}

func TestSMTMAMDeclarationIndexTracksScopeRollback(t *testing.T) {
	m, state, _, _ := smtMAMFixture(t)
	sort, err := m.MkSort(astTestSymbol(t, m, "enumeration-scope-sort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate := smtQuickPredicate(t, m, "enumeration-scope-p", sort, boolean)
	a, err := m.MkConst(astTestSymbol(t, m, "enumeration-base"), sort)
	astTestMust(t, err)
	base, err := m.MkApp(predicate, []ast.ASTHandle{a})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(base, false))
	it := newSMTMAMInterpreter(state, nil, false)
	before := it.enodesOf(predicate)
	if len(before) != 1 || before[0].owner != base {
		t.Fatal("base matching index", before)
	}
	astTestMust(t, state.PushScope())
	b, err := m.MkConst(astTestSymbol(t, m, "enumeration-scoped"), sort)
	astTestMust(t, err)
	scoped, err := m.MkApp(predicate, []ast.ASTHandle{b})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(scoped, false))
	during := it.enodesOf(predicate)
	allDuring := state.context.byDeclaration[predicate]
	if len(during) != 2 || during[0].owner != base || during[1].owner != scoped {
		t.Fatal("scoped matching order", during)
	}
	astTestMust(t, state.PopScopes(1))
	after := it.enodesOf(predicate)
	if len(after) != 1 || after[0].owner != base {
		t.Fatal("popped term remained in matching index", after)
	}
	if allDuring[1] != nil {
		t.Fatal("popped checker entry retained by backing array")
	}
	if during[1] != nil {
		t.Fatal("popped matching entry retained by backing array")
	}
	decl, err := m.GetDecl(a)
	astTestMust(t, err)
	if got := it.enodesOf(decl); len(got) != 0 {
		t.Fatal("constant was indexed as a matching application", got)
	}
}
