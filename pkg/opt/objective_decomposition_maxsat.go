package opt

import (
	"errors"
	"strconv"

	"fmt"
	utext "github.com/glycerine/g3/pkg/util/text"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// decompositionID is is_maxsat's `out << orig_term << ":" << index`: the
// expr_ref streams through mk_ismt2_pp, whose format tree lives only for
// that print.
func (d *ObjectiveDecomposer) decompositionID(source DecodedFormulaObjective, index int) (id string, err error) {
	if source.ID != "" {
		return source.ID, nil
	}
	if source.OriginalTerm == (ast.ASTHandle{}) {
		return fmt.Sprintf("null:%d", index), nil
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: d.manager.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	f, err := d.manager.SMT2PPExprTree(source.OriginalTerm, nil)
	if err != nil {
		return "", err
	}
	if err = d.manager.PrintFormatTransient(stream, f, nil, nil, nil); err != nil {
		return "", err
	}
	text, err := stream.String()
	if err != nil {
		return "", err
	}
	return text + ":" + strconv.Itoa(index), nil
}

func cloneRationals(source []big.Rat) []big.Rat {
	result := make([]big.Rat, len(source))
	for i := range source {
		result[i].Set(&source[i])
	}
	return result
}

func (d *ObjectiveDecomposer) retainedMaxSAT(source DecodedFormulaObjective) (DecomposedObjective, error) {
	result := DecomposedObjective{
		SourceKind: source.Kind, OriginalTerm: source.OriginalTerm,
		SoftTerms: make([]ast.ASTHandle, len(source.SoftTerms)),
		Weights:   cloneRationals(source.Weights), Offset: *new(big.Rat).Set(&source.Offset),
		Negate: source.Negate, ID: source.ID, Converted: true,
	}
	for i, term := range source.SoftTerms {
		retained, err := d.hold(term)
		if err != nil {
			return DecomposedObjective{}, err
		}
		result.SoftTerms[i] = retained
	}
	return result, nil
}

func (d *ObjectiveDecomposer) convertPBSum(source DecodedFormulaObjective, index int, sum pbSum) (DecomposedObjective, error) {
	result := DecomposedObjective{SourceKind: source.Kind, OriginalTerm: source.OriginalTerm, Converted: true}
	result.Offset.Set(&sum.offset)
	result.SoftTerms = make([]ast.ASTHandle, len(sum.terms))
	result.Weights = make([]big.Rat, len(sum.weights))
	for i := range sum.terms {
		weight := new(big.Rat).Set(&sum.weights[i])
		term := sum.terms[i]
		if source.Kind == FormulaMinimize {
			if weight.Sign() < 0 {
				result.Offset.Add(&result.Offset, weight)
				weight.Neg(weight)
			} else {
				var err error
				term, err = d.negated(term)
				if err != nil {
					return DecomposedObjective{}, err
				}
			}
		} else {
			if weight.Sign() < 0 {
				weight.Neg(weight)
				var err error
				term, err = d.negated(term)
				if err != nil {
					return DecomposedObjective{}, err
				}
			}
			result.Offset.Add(&result.Offset, weight)
			result.Negate = true
		}
		result.SoftTerms[i] = term
		result.Weights[i].Set(weight)
	}
	var err error
	if result.ID, err = d.decompositionID(source, index); err != nil {
		return DecomposedObjective{}, err
	}
	return result, nil
}
