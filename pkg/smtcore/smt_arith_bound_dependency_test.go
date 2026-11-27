package smtcore

import "testing"

func TestSMTArithmeticBoundDependencyJoinLinearizeAndContains(t *testing.T) {
	var manager smtArithmeticBoundDependencyManager
	first := &smtArithmeticBound{variable: 1}
	second := &smtArithmeticBound{variable: 2}
	left := manager.Leaf(first)
	right := manager.Leaf(second)
	root := manager.Join(left, manager.Join(right, left))
	if !manager.Contains(root, first) || !manager.Contains(root, second) || manager.Contains(root, &smtArithmeticBound{}) {
		t.Fatal("dependency containment")
	}
	values := manager.Linearize(root, nil)
	if len(values) != 2 || values[0] != first || values[1] != second {
		t.Fatalf("linearized dependencies = %#v", values)
	}
	if manager.Join(nil, left) != left || manager.Join(left, nil) != left || manager.Join(left, left) != left {
		t.Fatal("empty or idempotent join allocated a node")
	}
}

func TestSMTArithmeticBoundDependencyScopesAndReset(t *testing.T) {
	var manager smtArithmeticBoundDependencyManager
	manager.Leaf(&smtArithmeticBound{variable: 0})
	manager.PushScope()
	manager.Leaf(&smtArithmeticBound{variable: 1})
	manager.PushScope()
	manager.Leaf(&smtArithmeticBound{variable: 2})
	if !manager.PopScopes(2) || len(manager.nodes) != 1 || len(manager.scopes) != 0 {
		t.Fatal("dependency scope rollback")
	}
	if manager.PopScopes(1) || manager.PopScopes(0) {
		t.Fatal("invalid dependency scope rollback accepted")
	}
	manager.Reset()
	for _, node := range manager.nodes[:cap(manager.nodes)] {
		if node != nil {
			t.Fatal("reset retained a dependency in the backing array")
		}
	}
	if len(manager.nodes) != 0 || len(manager.scopes) != 0 {
		t.Fatal("dependency reset")
	}
}
