package fp_test

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func closeQueryTestRefs(manager *ast.ASTManager, references []ast.ASTRef) (result error) {
	for i := len(references) - 1; i >= 0; i-- {
		result = errors.Join(result, manager.DelRef(&references[i]))
	}
	return result
}
