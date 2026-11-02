package duality

// Counterexample exclusively owns its derivation graph; root access is borrowed.
// Source: duality.h:1118-1151. Go copies are rejected like the disabled C++ copy.
type Counterexample struct {
	self *Counterexample
	tree *RPFP
	root *Node
}

func NewCounterexample() *Counterexample { c := &Counterexample{}; c.self = c; return c }
func (c *Counterexample) open() error {
	if c == nil || c.self != c {
		return ErrWrapperOwner
	}
	return nil
}
func (c *Counterexample) Tree() *RPFP {
	if c == nil || c.self != c {
		return nil
	}
	return c.tree
}
func (c *Counterexample) Root() *Node {
	if c == nil || c.self != c {
		return nil
	}
	return c.root
}

// Set transfers a new graph only after releasing the previous graph. Passing
// the already-owned graph is rejected instead of creating a dangling owner.
func (c *Counterexample) Set(tree *RPFP, root *Node) error {
	if err := c.open(); err != nil {
		return err
	}
	if tree == nil && root == nil {
		return c.Clear()
	}
	if err := tree.open(); err != nil {
		return err
	}
	if root == nil || root.closed || root.Owner != tree || tree == c.tree {
		return ErrWrapperOwner
	}
	if err := c.Clear(); err != nil {
		return err
	}
	c.tree = tree
	c.root = root
	return nil
}
func (c *Counterexample) Swap(other *Counterexample) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	c.tree, other.tree = other.tree, c.tree
	c.root, other.root = other.root, c.root
	return nil
}
func (c *Counterexample) Clear() error {
	if err := c.open(); err != nil {
		return err
	}
	if c.tree != nil {
		if err := c.tree.Close(); err != nil {
			return err
		}
	}
	c.tree = nil
	c.root = nil
	return nil
}
func (c *Counterexample) Close() error {
	if c == nil {
		return nil
	}
	return c.Clear()
}
