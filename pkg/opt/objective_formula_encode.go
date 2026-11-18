package opt

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (c *ObjectiveFormulaCodec) EncodeObjective(objective FormulaObjective) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	var args []ast.ASTHandle
	metadata := objectiveFormulaMetadata{kind: objective.Kind, originalTerm: objective.Term, id: objective.ID, index: c.encoded}
	switch objective.Kind {
	case FormulaMaximize, FormulaMinimize:
		if objective.Term.IsNull() {
			return ast.ASTHandle{}, ast.ErrASTHandle
		}
		args = []ast.ASTHandle{objective.Term}
	case FormulaMaxSAT:
		if len(objective.SoftTerms) != len(objective.Weights) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		args = append([]ast.ASTHandle(nil), objective.SoftTerms...)
		weights, err := cloneRationals(c.manager.NumeralService().Manager, objective.Weights)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		metadata.weights = weights
		if diagnostic.VerboseProgress {
			for _, term := range objective.SoftTerms {
				ref, err := c.manager.NewRef(term)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				c.derived = append(c.derived, ref)
				metadata.softOriginal = append(metadata.softOriginal, term)
			}
		}
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	declaration, err := c.freshDeclaration(objective.Kind, args, metadata)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, releaseRationals(c.manager.NumeralService().Manager, metadata.weights))
	}
	c.encoded++
	return c.manager.MkApp(declaration, args)
}
