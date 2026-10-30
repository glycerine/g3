package interval

import (
	"reflect"
	"testing"
)

func TestDependentIntervalDependencyJoinLinearization(t *testing.T) {
	m := &IntervalDependencyManager{}
	one, two, three := m.Leaf(1), m.Leaf(2), m.Leaf(3)
	shared := m.Join(one, two)
	root := m.Join(m.Join(shared, three), m.Join(shared, two))
	if got := m.Linearize(root); !reflect.DeepEqual(got, []uint32{3, 2, 1}) {
		t.Fatal(got)
	}
	if !m.Contains(root, 2) || m.Contains(root, 9) {
		t.Fatal("contains")
	}
	if m.Join(nil, one) != one || m.Join(one, one) != one {
		t.Fatal("join identities")
	}
}
