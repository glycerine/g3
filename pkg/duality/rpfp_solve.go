package duality

// Source: duality_rpfp.cpp:1343-1372,1418-1420.
import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (g *RPFP) GetConstraintTree(root, skip *Node) (*TermTree, error) {
	tree, err := g.ToTermTree(root, skip)
	if err != nil {
		return nil, err
	}
	wrapped, err := g.AddUpperBound(root, tree)
	if err != nil {
		return nil, errors.Join(err, tree.Close())
	}
	return wrapped, nil
}

// Solve interpolates the currently asserted graph and updates its node
// annotations on UNSAT. SAT leaves the acquired counterexample model owned
// by the graph. The source persist argument affects only disabled code in
// DecodeTree and is therefore unnecessary here.
func (g *RPFP) Solve(root *Node, manager *interp.Manager, config *interp.RunConfig) (status lbool.LBool, result error) {
	tree, err := g.GetConstraintTree(root, nil)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { result = errors.Join(result, tree.Close()) }()
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
		result = g.DecodeTree(root, learned.children[0])
	}
	return
}
