package realclosure

// Source: src/math/realclosure/realclosure.cpp:724-727, 1808-1835.

func (o *algebraicRootOwner) registerAlgebraicRoot(root *algebraicRoot) error {
	index, err := o.nextAlgebraicIndex()
	if err != nil {
		return err
	}
	root.index = index
	o.roots = append(o.roots, root)
	return nil
}
