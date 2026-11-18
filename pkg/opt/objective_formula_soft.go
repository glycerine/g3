package opt

import (
	"github.com/glycerine/g3/pkg/util/numeral"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// normalizeSoft returns caller-owned weights and offset.
func (c *ObjectiveFormulaCodec) normalizeSoft(args []ast.ASTHandle, metadata objectiveFormulaMetadata) (terms []ast.ASTHandle, weights []numeral.Rational, offset numeral.Rational, err error) {
	o := newRatOps(c.manager)
	offset = o.zero()
	if len(args) != len(metadata.weights) {
		return nil, nil, offset, ast.ErrASTDomain
	}
	weight := o.zero()
	defer func() {
		o.del(&weight)
		if err = o.joined(err); err != nil {
			o.delAll(weights)
			weights = nil
		}
	}()
	for i, argument := range args {
		o.set(&weight, &metadata.weights[i])
		if o.sign(&weight) < 0 {
			o.neg(&weight)
			o.sub(&offset, &offset, &weight)
			// is_maxsat: arg = mk_not(m, arg) (ast_util: strips a leading
			// not and folds the Boolean constants).
			argument, err = c.manager.MkNotSimplified(argument)
			if err != nil {
				return nil, weights, offset, err
			}
			if !c.manager.IsTrue(argument) && !c.manager.IsFalse(argument) {
				ref, refErr := c.manager.NewRef(argument)
				if refErr != nil {
					return nil, weights, offset, refErr
				}
				c.derived = append(c.derived, ref)
			}
		}
		// is_maxsat: IF_VERBOSE(1, weight << ": " << mk_pp(m_terms[i]) <<
		// " |-> true|false") for a term folded to a constant (AUDIT.md
		// FUZZ-F5; g3trace builds only).
		if c.manager.IsTrue(argument) {
			if err = c.softFoldVerbose(metadata, i, &weight, "true"); err != nil {
				return nil, weights, offset, err
			}
			continue
		}
		if o.sign(&weight) == 0 {
			continue
		}
		if c.manager.IsFalse(argument) {
			if err = c.softFoldVerbose(metadata, i, &weight, "false"); err != nil {
				return nil, weights, offset, err
			}
			o.add(&offset, &offset, &weight)
			continue
		}
		terms = append(terms, argument)
		weights = append(weights, o.clone(&weight))
	}
	return terms, weights, offset, nil
}

func (c *ObjectiveFormulaCodec) softFoldVerbose(metadata objectiveFormulaMetadata, i int, weight *numeral.Rational, value string) error {
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
	o := newRatOps(c.manager)
	weightText := o.text(weight)
	if !o.ok() {
		return o.err
	}
	return optProgress(1, weightText+": "+text+" |-> "+value+"\n")
}
