package opt

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *ObjectiveFormulaCodec) normalizeSoft(args []ast.ASTHandle, metadata objectiveFormulaMetadata) (terms []ast.ASTHandle, weights []big.Rat, offset big.Rat, err error) {
	if len(args) != len(metadata.weights) {
		return nil, nil, offset, ast.ErrASTDomain
	}
	for i, argument := range args {
		weight := metadata.weights[i]
		if weight.Sign() < 0 {
			weight.Neg(&weight)
			offset.Sub(&offset, &weight)
			// is_maxsat: arg = mk_not(m, arg) (ast_util: strips a leading
			// not and folds the Boolean constants).
			argument, err = c.manager.MkNotSimplified(argument)
			if err != nil {
				return nil, nil, offset, err
			}
			if !c.manager.IsTrue(argument) && !c.manager.IsFalse(argument) {
				ref, refErr := c.manager.NewRef(argument)
				if refErr != nil {
					return nil, nil, offset, refErr
				}
				c.derived = append(c.derived, ref)
			}
		}
		if weight.Sign() == 0 || c.manager.IsTrue(argument) {
			continue
		}
		if c.manager.IsFalse(argument) {
			offset.Add(&offset, &weight)
			continue
		}
		terms = append(terms, argument)
		weights = append(weights, weight)
	}
	return terms, weights, offset, nil
}
