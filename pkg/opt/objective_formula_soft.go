package opt

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
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
		// is_maxsat: IF_VERBOSE(1, weight << ": " << mk_pp(m_terms[i]) <<
		// " |-> true|false") for a term folded to a constant (AUDIT.md
		// FUZZ-F5; g3trace builds only).
		if c.manager.IsTrue(argument) {
			if err = c.softFoldVerbose(metadata, i, &weight, "true"); err != nil {
				return nil, nil, offset, err
			}
			continue
		}
		if weight.Sign() == 0 {
			continue
		}
		if c.manager.IsFalse(argument) {
			if err = c.softFoldVerbose(metadata, i, &weight, "false"); err != nil {
				return nil, nil, offset, err
			}
			offset.Add(&offset, &weight)
			continue
		}
		terms = append(terms, argument)
		weights = append(weights, weight)
	}
	return terms, weights, offset, nil
}

func (c *ObjectiveFormulaCodec) softFoldVerbose(metadata objectiveFormulaMetadata, i int, weight *big.Rat, value string) error {
	if !diagnostic.VerboseProgress || i >= len(metadata.softOriginal) {
		return nil
	}
	if enabled, err := optProgressEnabled(1); err != nil || !enabled {
		return err
	}
	text, err := mkPPStringOf(c.manager, metadata.softOriginal[i])
	if err != nil {
		return err
	}
	return optProgress(1, ratText(weight)+": "+text+" |-> "+value+"\n")
}
