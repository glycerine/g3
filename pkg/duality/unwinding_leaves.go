package duality

// Source: duality_solver.cpp:516-541,974-989.

// CreateLeaf builds the source do_not_expand leaf without invoking eager
// expansion. Its lower-bound edge and final annotation are both false.
func (u *Unwinding) CreateLeaf(source *Node) (*Node, error) {
	node, err := u.CreateNodeInstance(source, 0)
	if err != nil {
		return nil, err
	}
	if err = node.Annotation.SetEmpty(); err != nil {
		return nil, err
	}
	edge, err := u.graph.CreateLowerBoundEdge(node)
	if err != nil {
		return nil, err
	}
	edge.Map = nil
	if u.StratifiedInlining {
		if err = node.Annotation.SetFull(); err != nil {
			return nil, err
		}
	} else {
		if u.updatedNodes == nil {
			u.updatedNodes = make(map[*Node]bool)
		}
		u.updatedNodes[node] = true
	}
	if err = node.Annotation.SetEmpty(); err != nil {
		return nil, err
	}
	return node, nil
}
func (u *Unwinding) MakeOverapprox(node *Node) error {
	if u == nil || node == nil || node.Owner != u.graph || node.closed {
		return ErrWrapperOwner
	}
	if err := node.Annotation.SetFull(); err != nil {
		return err
	}
	edge, err := u.graph.CreateLowerBoundEdge(node)
	if err != nil {
		return err
	}
	edge.Map = nil
	if u.overapproximations == nil {
		u.overapproximations = make(map[*Node]bool)
	}
	u.overapproximations[node] = true
	return nil
}

// MarkExpanded activates only nodes still in the source pending set. Repeated
// calls do not append duplicate active instances.
func (u *Unwinding) MarkExpanded(node *Node) error {
	if u == nil || node == nil || node.Owner != u.graph || node.closed {
		return ErrWrapperOwner
	}
	for _, pending := range u.unexpanded {
		if pending.Number == node.Number {
			return u.ActivateInstance(node)
		}
	}
	return nil
}
