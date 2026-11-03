package pdr

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDRProjectedChildrenPreserveFourPredecessorSignatures(t *testing.T) {
	m, context, rules := classifierContextFixture(t)
	var pins []ast.ASTRef
	t.Cleanup(func() { closureMust(t, closeASTRefs(m, pins)) })
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		closureMust(t, err)
		ref, err := m.NewRef(h)
		closureMust(t, err)
		pins = append(pins, ref)
		return h
	}
	boolean := own(m.MkBoolSort())
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		symbol, err := m.Symbols.Intern(name)
		closureMust(t, err)
		declaration := own(m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{}))
		closureMust(t, rules.RegisterPredicate(declaration))
		return declaration
	}
	goal := declare("projected-many-goal", nil)
	head := own(m.MkApp(goal, nil))
	set, err := base.NewRuleSet(rules)
	closureMust(t, err)
	t.Cleanup(func() { closureMust(t, set.Close()) })
	add := func(head ast.ASTHandle, tail []ast.ASTHandle, name string) {
		t.Helper()
		rule, err := rules.NewRule(head, tail, nil, name)
		closureMust(t, err)
		closureMust(t, set.Add(rule))
		closureMust(t, rule.Close())
	}
	var predecessors, tail []ast.ASTHandle
	for i := uint32(0); i < 4; i++ {
		predicate := declare(fmt.Sprintf("projected-many-P%d", i), []ast.ASTHandle{boolean, boolean})
		predecessors = append(predecessors, predicate)
		x, y := own(m.MkVar(2*i, boolean)), own(m.MkVar(2*i+1, boolean))
		application := own(m.MkApp(predicate, []ast.ASTHandle{x, y}))
		add(application, nil, fmt.Sprintf("projected-many-fact-%d", i))
		tail = append(tail, application, own(m.MkNot(x)), own(m.MkNot(y)))
	}
	add(head, tail, "projected-many-join")
	closureMust(t, context.Initialize(set))
	status, err := context.Solve(goal)
	closureMust(t, err)
	if status != lbool.True {
		t.Fatalf("four-predecessor query = %v, want true", status)
	}
	root, found, err := context.search.Root()
	closureMust(t, err)
	if !found || !root.IsClosed() || len(root.children) != len(predecessors) {
		t.Fatalf("closed four-predecessor derivation missing: found=%v root=%v", found, root)
	}
	// create_children partitions old-state literals, then converts each cube
	// to its predecessor's new-state signature (pdr_context.cpp:2310-2320).
	for i, child := range root.children {
		if child.parent != root || child.level != 0 || !child.IsClosed() || child.transformer.head != predecessors[i] {
			t.Fatalf("child %d has the wrong predecessor or search state", i)
		}
		literals, err := m.FlattenAnd([]ast.ASTHandle{child.state})
		closureMust(t, err)
		if len(literals) != 2 {
			t.Fatalf("child %d has %d state literals, want 2", i, len(literals))
		}
		seen := make(map[ast.ASTHandle]bool, len(literals))
		for _, literal := range literals {
			if seen[literal] {
				t.Fatalf("child %d repeats a state literal", i)
			}
			seen[literal] = true
		}
		for j, signature := range child.transformer.signature {
			declaration, err := context.owner.OldToNew(signature, 0)
			closureMust(t, err)
			application := own(m.MkApp(declaration, nil))
			want := own(m.MkNot(application))
			if !seen[want] {
				t.Fatalf("child %d lacks negative signature %d: %v", i, j, want)
			}
		}
	}
}
