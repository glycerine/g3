package opt

// Source: src/opt/sortmax.cpp m_trail ownership.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type maxSMTASTOwner struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func (o *maxSMTASTOwner) hold(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := o.manager.NewRef(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	o.refs = append(o.refs, ref)
	return value, nil
}

// close is the engines' expr_ref_vector members (m_asms, m_trail, ...)
// going away: they release front to back.
func (o *maxSMTASTOwner) close() (result error) {
	for i := range o.refs {
		result = errors.Join(result, o.manager.DelRef(&o.refs[i]))
	}
	o.refs = nil
	return result
}
