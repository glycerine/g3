package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// A sliced-away closure still keeps all captured enodes reachable to Go's GC.
func TestSMTMAMTrailReleasesUndoneClosures(t *testing.T) {
	var trail smtMAMTrail
	var order []int
	trail.push(func() { order = append(order, 0) })
	trail.pushScope()
	trail.push(func() { order = append(order, 1) })
	trail.pushScope()
	trail.push(func() { order = append(order, 2) })
	backing := trail.undo
	trail.popScope(1)
	if len(order) != 1 || order[0] != 2 || backing[0] == nil || backing[1] == nil {
		t.Fatal("incorrect nested undo", order)
	}
	if backing[2] != nil {
		t.Error("popped closure retained in trail backing array")
	}
	trail.reset()
	if len(order) != 3 || order[1] != 1 || order[2] != 0 {
		t.Fatal("incorrect reset order", order)
	}
	for _, undo := range backing {
		if undo != nil {
			t.Error("reset retained an undo closure")
		}
	}
	trail.reset()
	if len(order) != 3 {
		t.Fatal("reset repeated an undo")
	}
}

func TestSMTMAMCandidateResetReleasesEnodes(t *testing.T) {
	tree := smtMAMCodeTree{}
	tree.addCandidate(&SMTEnode{})
	backing := tree.candidates
	tree.resetCandidates()
	if len(tree.candidates) != 0 || backing[0] != nil {
		t.Fatal("reset candidate list retains an enode")
	}
}

// Exercise a successful selective join: p(a) is a smaller bucket than all p
// applications. The callback consumes its borrowed bindings synchronously,
// as registry.AddInstanceUsed does. Repeating this join must reuse scratch.
func TestSMTMAMSelectiveJoinReusesScratch(t *testing.T) {
	m, state, _, _ := smtMAMFixture(t)
	sort, e := m.MkSort(astTestSymbol(t, m, "scratch-sort"), ast.ASTSortInfo{})
	astTestMust(t, e)
	boolean, e := m.MkBoolSort()
	astTestMust(t, e)
	pred := smtQuickPredicate(t, m, "scratch-p", sort, boolean)
	_, a := smtEnodeConstant(t, m, 34000, sort, 0)
	astTestMust(t, state.context.RegisterEnode(a, true))
	_, b := smtEnodeConstant(t, m, 34001, sort, 0)
	astTestMust(t, state.context.RegisterEnode(b, true))
	_, pa := smtQuickApplicationNode(t, m, state.context, pred, a, true)
	_, pb := smtQuickApplicationNode(t, m, state.context, pred, b, true)
	a.sourceParents = []*SMTEnode{pa}
	b.sourceParents = []*SMTEnode{pb}
	matches := 0
	it := newSMTMAMInterpreter(state, func(_, _ ast.ASTHandle, bs []*SMTEnode, _ uint32, _ []*SMTEnode) error {
		if len(bs) != 1 || bs[0] != a {
			t.Fatal("wrong join binding", bs)
		}
		matches++
		return nil
	}, false)
	yield := &smtMAMInstr{op: mamYIELD1, binding: []uint32{2}}
	cont := &smtMAMInstr{op: mamCONTINUE, label: pred, numArgs: 1, joints: []smtMAMJoint{{tag: mamVarTag, reg: 1}}, oreg: 2, next: yield}
	tree := &smtMAMCodeTree{numRegs: 3, numChoices: 1, root: &smtMAMInstr{op: mamGETENODE, oreg: 1, enode: a, next: cont}, candidates: []*SMTEnode{pb}}
	astTestMust(t, it.execute(tree))
	if matches != 1 {
		t.Fatal("join did not run", matches)
	}
	allocs := testing.AllocsPerRun(30, func() {
		if err := it.execute(tree); err != nil {
			panic(err)
		}
	})
	if allocs != 0 {
		t.Errorf("selective join allocates %.0f times per match; want reusable scratch", allocs)
	}
	checkReleased := func() {
		t.Helper()
		for _, n := range it.registers {
			if n != nil {
				t.Error("matcher retains a register enode")
				break
			}
		}
		for _, n := range it.bindings {
			if n != nil {
				t.Error("matcher retains a binding enode")
				break
			}
		}
		for _, bp := range it.backtrack {
			if bp.candidates != nil {
				t.Error("matcher retains backtracking candidates")
			}
		}
	}
	checkReleased()
	stop := errors.New("stop matching")
	it.onMatch = func(_, _ ast.ASTHandle, _ []*SMTEnode, _ uint32, _ []*SMTEnode) error { return stop }
	if err := it.execute(tree); !errors.Is(err, stop) {
		t.Fatal("lost callback error", err)
	}
	checkReleased()
	it.cancelled = func() bool { return true }
	astTestMust(t, it.execute(tree))
	checkReleased()
}
