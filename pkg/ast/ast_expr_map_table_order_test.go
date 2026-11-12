package ast

import "testing"

// expr_map iterates (Pairs) and releases (Reset) in obj_map table order:
// the order of an ObjMap that saw the same inserts and erases.
func TestASTExprMapPairsFollowTableOrder(t *testing.T) {
	m := astTestManager(t)
	nodes := markTestNodes(t, m, 40)
	for _, h := range nodes {
		astTestMust(t, m.IncRef(h))
	}
	s, e := NewASTExprMapWithProofs(m, false)
	astTestMust(t, e)
	reference := NewObjMap(m)
	for i := len(nodes) - 1; i >= 0; i-- {
		astTestMust(t, s.Insert(nodes[i], nodes[(i+1)%len(nodes)], ASTHandle{}))
		astTestMust(t, reference.Insert(nodes[i], [2]ASTHandle{}))
	}
	for i := 0; i < len(nodes); i += 3 {
		astTestMust(t, s.Erase(nodes[i]))
		astTestMust(t, reference.Erase(nodes[i]))
	}
	pairs, e := s.Pairs()
	astTestMust(t, e)
	want := reference.Entries()
	if len(pairs) != len(want) {
		t.Fatalf("pairs %d, want %d", len(pairs), len(want))
	}
	for i := range pairs {
		if pairs[i].Key != want[i].Key {
			t.Fatalf("pair %d out of table order", i)
		}
	}
	astTestMust(t, s.Close())
}
