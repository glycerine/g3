package opt

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (d *ObjectiveDecomposer) appKind(h ast.ASTHandle, family ast.FamilyID, kind int32) (bool, error) {
	k, err := d.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := d.manager.GetFamilyID(h)
	if err != nil || f != family {
		return false, err
	}
	op, err := d.manager.GetDeclKind(h)
	return op == kind, err
}

func (d *ObjectiveDecomposer) args(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	n, err := d.manager.GetNumArgs(h)
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		result[i], err = d.manager.GetArg(h, i)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
