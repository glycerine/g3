package duality

// Source: duality_rpfp.cpp:1373-1415.
import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// CollapseTermTree appends descendants' main and extra terms in source
// preorder to the root, then releases the descendant tree objects.
func CollapseTermTree(root *TermTree) error {
	if err := root.open(); err != nil {
		return err
	}
	var collect func(*TermTree) error
	collect = func(node *TermTree) error {
		if err := node.open(); err != nil {
			return err
		}
		if err := root.AddTerm(node.term); err != nil {
			return err
		}
		for _, term := range node.terms {
			if err := root.AddTerm(term); err != nil {
				return err
			}
		}
		for _, child := range node.children {
			if err := collect(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, child := range root.children {
		if err := collect(child); err != nil {
			return err
		}
	}
	for len(root.children) > 0 {
		n := len(root.children) - 1
		if err := root.children[n].Close(); err != nil {
			return err
		}
		root.children = root.children[:n]
	}
	return nil
}

// SolveSingleNode separates one subtree from its collapsed surrounding
// context, learns its interpolant, and updates only that node's annotation.
func (g *RPFP) SolveSingleNode(root, node *Node, manager *interp.Manager, config *interp.RunConfig) (status lbool.LBool, result error) {
	if err := g.open(); err != nil {
		return lbool.Undef, err
	}
	if node == nil || node.Owner != g || node.closed {
		return lbool.Undef, ErrWrapperOwner
	}
	tree, err := g.GetConstraintTree(root, node)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { result = errors.Join(result, tree.Close()) }()
	if err = CollapseTermTree(tree); err != nil {
		return lbool.Undef, err
	}
	subtree, err := g.ToTermTree(node, nil)
	if err != nil {
		return lbool.Undef, err
	}
	tree.children = append(tree.children, subtree)
	if err = CollapseTermTree(subtree); err != nil {
		return lbool.Undef, err
	}
	if err = g.ClearProofCore(); err != nil {
		return lbool.Undef, err
	}
	status, learned, err := g.InterpolateTree(manager, tree, nil, true, config)
	if learned != nil {
		defer func() { result = errors.Join(result, learned.Close()) }()
	}
	if err != nil {
		return status, err
	}
	if status == lbool.False {
		if learned == nil || len(learned.children) != 1 {
			return status, ErrWrapperOwner
		}
		result = g.DecodeTree(node, learned.children[0])
	}
	return
}
