package smtcore

import "testing"

// pool<enode_vector>: mk reuses the most recently recycled vector (LIFO,
// as pool.h's pop_back), and a recycled vector holds no enodes.
func TestSMTMAMEnodeVectorPoolReusesClearedVectors(t *testing.T) {
	var p smtMAMEnodeVectorPool
	a, b := p.mk(), p.mk()
	nodes := []*SMTEnode{{}, {}, {}}
	*a = append(*a, nodes...)
	*b = append(*b, nodes[0])
	p.recycle(a)
	p.recycle(b)
	for _, v := range []*[]*SMTEnode{a, b} {
		if len(*v) != 0 {
			t.Fatal("recycled vector not reset")
		}
		for _, n := range (*v)[:cap(*v)] {
			if n != nil {
				t.Fatal("recycled vector retains an enode")
			}
		}
	}
	if p.mk() != b || p.mk() != a || len(p.objs) != 0 {
		t.Fatal("pool order")
	}
	if p.mk() == nil {
		t.Fatal("empty pool must allocate")
	}
}
