package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// hold pushes onto nnf::m_trail.
func (n *QENNFNormalizer) hold(expression ast.ASTHandle) error {
	ref, err := n.manager.NewRef(expression)
	if err != nil {
		return err
	}
	n.scratch = append(n.scratch, ref)
	return nil
}

// releaseForward is expr_ref_vector::reset: references drop in order.
func (n *QENNFNormalizer) releaseForward(refs *[]ast.ASTRef) error {
	for i := range *refs {
		if (*refs)[i] != (ast.ASTRef{}) {
			if err := n.manager.DelRef(&(*refs)[i]); err != nil {
				*refs = (*refs)[i:]
				return err
			}
		}
	}
	*refs = nil
	return nil
}

func (n *QENNFNormalizer) resetCall() (result error) {
	result = errors.Join(n.releaseForward(&n.literalTrail), n.releaseForward(&n.scratch))
	n.memo = make(map[qeNNFKey]ast.ASTHandle)
	n.todo = n.todo[:0]
	return result
}
