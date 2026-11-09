package smtcore

import "testing"

func TestSMTModelFinderAUFNodeMergeConstraintsAndInstantiationSet(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "model-finder-node-sort"), nil)
	astTestMust(t, err)
	left, err := newSMTModelFinderAUFNode(0, sort)
	astTestMust(t, err)
	right, err := newSMTModelFinderAUFNode(1, sort)
	astTestMust(t, err)
	avoid, err := newSMTModelFinderAUFNode(2, sort)
	astTestMust(t, err)
	exception, err := m.MkConst(astTestSymbol(t, m, "model-finder-node-exception"), sort)
	astTestMust(t, err)
	astTestMust(t, left.setMono())
	astTestMust(t, left.insertAvoid(avoid))
	astTestMust(t, left.insertException(exception))
	astTestMust(t, right.setSigned())
	astTestMust(t, left.merge(right))
	root := left.root()
	if root != right.root() || !root.mono || !root.signed || root.size != 2 || len(root.avoid) != 1 || len(root.exceptions) != 1 {
		t.Fatal("merged AUF node constraints", root)
	}
	astTestMust(t, avoid.merge(root))
	if !root.mustAvoidItself() {
		t.Fatal("merged avoid class did not create a self-avoid constraint")
	}
	astTestMust(t, root.makeInstantiationSet(m))
	term, err := m.MkConst(astTestSymbol(t, m, "model-finder-node-term"), sort)
	astTestMust(t, err)
	inserted, err := root.insert(term, 7)
	astTestMust(t, err)
	if !inserted || root.set.testElements()[term] != 7 {
		t.Fatal("AUF node instantiation set", inserted, root.set.testElements())
	}
	if err = left.merge(right); err != nil {
		t.Fatal("same-class merge after set creation", err)
	}
	astTestMust(t, root.close())
}
