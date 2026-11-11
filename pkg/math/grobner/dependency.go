package grobner

// Source: src/util/dependency.h, the v_dependency_manager
// (scoped_dependency_manager<void*>) that grobner shares with theory_arith.
// The source leaves are bound pointers; here a leaf holds the index the
// owning theory assigns to the bound. Nodes need no reference counts: the
// scoped manager releases them with its region.

// Dependency is a v_dependency: a leaf or the join of two dependencies. A
// nil Dependency is the empty dependency.
type Dependency struct {
	leaf        int
	left, right *Dependency
	marked      bool
}

// DependencyLeaf is mk_leaf(v) for the bound with the given index.
func DependencyLeaf(index int) *Dependency {
	return &Dependency{leaf: index}
}

// DependencyJoin is mk_join(d1, d2).
func DependencyJoin(d1, d2 *Dependency) *Dependency {
	switch {
	case d1 == nil:
		return d2
	case d2 == nil:
		return d1
	case d1 == d2:
		return d1
	}
	return &Dependency{left: d1, right: d2}
}

func (d *Dependency) isLeaf() bool { return d.left == nil }

// Linearize is linearize(d, vs): the leaves in breadth-first order, each
// node visited once.
func Linearize(d *Dependency) []int {
	if d == nil {
		return nil
	}
	var values []int
	d.marked = true
	todo := []*Dependency{d}
	for qhead := 0; qhead < len(todo); qhead++ {
		d = todo[qhead]
		if d.isLeaf() {
			values = append(values, d.leaf)
			continue
		}
		for _, child := range [2]*Dependency{d.left, d.right} {
			if !child.marked {
				todo = append(todo, child)
				child.marked = true
			}
		}
	}
	for _, n := range todo {
		n.marked = false
	}
	return values
}
