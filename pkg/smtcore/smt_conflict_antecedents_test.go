package smtcore

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTAntecedentExpanderNestedJustificationOrderAndCleanup(t *testing.T) {
	child, err := NewSMTSimpleJustification(true, []SMTLiteral{NewSMTLiteral(3, false)})
	if err != nil {
		t.Fatal(err)
	}
	parent, err := NewSMTUnitResolutionJustification(true, child, []SMTLiteral{NewSMTLiteral(4, true)})
	if err != nil {
		t.Fatal(err)
	}
	expander := NewSMTAntecedentExpander()
	result, err := expander.ExpandJustification(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || result[0] != NewSMTLiteral(4, true) || result[1] != NewSMTLiteral(3, false) {
		t.Fatal("source queue order", result)
	}
	if parent.IsMarked() || child.IsMarked() {
		t.Fatal("marks were not cleared")
	}
	result, err = expander.ExpandJustification(parent)
	if err != nil || len(result) != 2 {
		t.Fatal("reusable expansion", result, err)
	}
}

func TestSMTAntecedentExpanderTransitivityEquationsAndCommonAncestor(t *testing.T) {
	manager := astTestManager(t)
	sort, err := manager.MkSort(symbol.Numerical(2800), ast.ASTSortInfo{})
	if err != nil {
		t.Fatal(err)
	}
	_, left := smtEnodeConstant(t, manager, 2801, sort, 0)
	_, right := smtEnodeConstant(t, manager, 2802, sort, 0)
	_, root := smtEnodeConstant(t, manager, 2803, sort, 0)
	left.root, right.root = root, root
	left.trans = newSMTEqualityEdge(nil, root, NewSMTEqualityEquation(NewSMTLiteral(7, false)))
	right.trans = newSMTEqualityEdge(nil, root, NewSMTEqualityEquation(NewSMTLiteral(8, true)))
	expander := NewSMTAntecedentExpander()
	result, err := expander.ExpandEquality(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || result[0] != NewSMTLiteral(7, false) || result[1] != NewSMTLiteral(8, true) {
		t.Fatal("transitivity literals", result)
	}
	uses := expander.EqualityUses()
	if len(uses) != 1 || uses[0].Common != root {
		t.Fatal("common ancestor event", uses)
	}
}

func TestSMTAntecedentExpanderCongruenceQueuesArgumentEqualities(t *testing.T) {
	manager := astTestManager(t)
	sort, err := manager.MkSort(symbol.Numerical(2810), ast.ASTSortInfo{})
	if err != nil {
		t.Fatal(err)
	}
	a, an := smtEnodeConstant(t, manager, 2811, sort, 0)
	b, bn := smtEnodeConstant(t, manager, 2812, sort, 0)
	a2, an2 := smtEnodeConstant(t, manager, 2813, sort, 0)
	b2, bn2 := smtEnodeConstant(t, manager, 2814, sort, 0)
	an2.root, an2.trans = an, newSMTEqualityEdge(nil, an, NewSMTEqualityAxiom())
	bn2.root, bn2.trans = bn, newSMTEqualityEdge(nil, bn, NewSMTEqualityAxiom())
	decl := smtCGTestDeclaration(t, manager, 2815, sort, ast.ASTFuncFlags{})
	first := smtCGTestApplication(t, manager, decl, []ast.ASTHandle{a, b}, []*SMTEnode{an, bn})
	second := smtCGTestApplication(t, manager, decl, []ast.ASTHandle{a2, b2}, []*SMTEnode{an2, bn2})
	first.root = second
	first.trans = newSMTEqualityEdge(nil, second, NewSMTEqualityCongruence(false))
	expander := NewSMTAntecedentExpander()
	result, err := expander.ExpandEquality(first, second)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 || len(expander.CongruenceUses()) != 1 || len(expander.EqualityUses()) != 3 {
		t.Fatal("congruence expansion", result, expander.CongruenceUses(), expander.EqualityUses())
	}
}

// Return only the edge chain so the GC must retain the solver-owned literal
// page and detached proof object through their tagged interior pointers.
func smtTestDetachedEqualityChain(t *testing.T) []*SMTEnode {
	t.Helper()
	state := SMTInternalizerState{boolExpr: make([]ast.ASTHandle, 1024)}
	theory, err := NewSMTSimpleJustification(false, []SMTLiteral{9191})
	astTestMust(t, err)
	reasons := []SMTEqualityJustification{
		NewSMTEqualityAxiom(), NewSMTEqualityCongruence(false), NewSMTEqualityCongruence(true),
		NewSMTEqualityEquation(0), NewSMTEqualityEquation(511), NewSMTEqualityEquation(-1 << 31),
		NewSMTEqualityEquation(1<<31 - 1), NewSMTEqualityJustification(theory),
		NewSMTEqualityJustification((*SMTSimpleJustification)(nil)),
	}
	nodes := make([]*SMTEnode, len(reasons)+1)
	for i := range nodes {
		nodes[i] = new(SMTEnode)
	}
	for i, j := range reasons {
		nodes[i].trans = newSMTEqualityEdge(&state, nodes[i+1], j)
	}
	return nodes
}

func TestSMTEqualityEdgeLayoutAndReversalAcrossGC(t *testing.T) {
	word := unsafe.Sizeof(uintptr(0))
	if got := unsafe.Sizeof(smtEqualityEdge{}); got != 2*word {
		t.Fatalf("stored equality edge is %d bytes, want %d", got, 2*word)
	}
	nodes := smtTestDetachedEqualityChain(t)
	runtime.GC()
	runtime.GC()
	proof, ok := nodes[7].trans.justification().Justification()
	if !ok || proof == nil || proof.(*SMTSimpleJustification).Literals[0] != 9191 {
		t.Fatal("lost detached proof object")
	}
	want := []SMTEqualityJustification{
		NewSMTEqualityAxiom(), NewSMTEqualityCongruence(false), NewSMTEqualityCongruence(true),
		NewSMTEqualityEquation(0), NewSMTEqualityEquation(511), NewSMTEqualityEquation(-1 << 31),
		NewSMTEqualityEquation(1<<31 - 1), NewSMTEqualityJustification(proof),
		NewSMTEqualityJustification((*SMTSimpleJustification)(nil)),
	}
	last := len(nodes) - 1
	check := func(reverse bool) {
		t.Helper()
		root := last
		if reverse {
			root = 0
		}
		if e := nodes[root].trans; e.Target != nil || e.justification() != NullSMTEqualityJustification() {
			t.Fatal("root has a proof edge")
		}
		for i, r := range want {
			from, to := i, i+1
			if reverse {
				from, to = to, from
			}
			if e := nodes[from].trans; e.Target != nodes[to] || e.justification() != r {
				t.Fatalf("edge %d to %d lost its reason", from, to)
			}
		}
	}
	check(false)
	smtInvertTrans(nodes[0])
	runtime.GC()
	check(true)
	smtInvertTrans(nodes[last])
	check(false)
}
