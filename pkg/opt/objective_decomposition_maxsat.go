package opt

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"
	"strconv"

	"fmt"
	utext "github.com/glycerine/g3/pkg/util/text"

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

// The result owns its rationals.
func (d *ObjectiveDecomposer) retainedMaxSAT(source DecodedFormulaObjective) (DecomposedObjective, error) {
	q := d.manager.NumeralService().Manager
	weights, err := cloneRationals(q, source.Weights)
	if err != nil {
		return DecomposedObjective{}, err
	}
	offset, err := cloneRational(q, &source.Offset)
	result := DecomposedObjective{
		SourceKind: source.Kind, OriginalTerm: source.OriginalTerm,
		SoftTerms: make([]ast.ASTHandle, len(source.SoftTerms)),
		Weights:   weights, Offset: offset,
		Negate: source.Negate, ID: source.ID, Converted: true,
	}
	if err != nil {
		return DecomposedObjective{}, errors.Join(err, result.ReleaseRationals(q))
	}
	for i, term := range source.SoftTerms {
		retained, err := d.hold(term)
		if err != nil {
			return DecomposedObjective{}, errors.Join(err, result.ReleaseRationals(q))
		}
		result.SoftTerms[i] = retained
	}
	return result, nil
}

// convertPBSum borrows sum; the result owns its rationals.
func (d *ObjectiveDecomposer) convertPBSum(source DecodedFormulaObjective, index int, sum pbSum) (_ DecomposedObjective, err error) {
	o := newRatOps(d.manager)
	result := DecomposedObjective{SourceKind: source.Kind, OriginalTerm: source.OriginalTerm, Converted: true, Offset: o.clone(&sum.offset)}
	result.SoftTerms = make([]ast.ASTHandle, len(sum.terms))
	result.Weights = make([]numeral.Rational, len(sum.weights))
	for i := range result.Weights {
		result.Weights[i] = numeral.NewRational()
	}
	defer func() {
		if err = o.joined(err); err != nil {
			err = errors.Join(err, result.ReleaseRationals(o.q))
		}
	}()
	for i := range sum.terms {
		weight := &result.Weights[i]
		o.set(weight, &sum.weights[i])
		term := sum.terms[i]
		if source.Kind == FormulaMinimize {
			if o.sign(weight) < 0 {
				o.add(&result.Offset, &result.Offset, weight)
				o.neg(weight)
			} else {
				term, err = d.negated(term)
				if err != nil {
					return DecomposedObjective{}, err
				}
			}
		} else {
			if o.sign(weight) < 0 {
				o.neg(weight)
				term, err = d.negated(term)
				if err != nil {
					return DecomposedObjective{}, err
				}
			}
			o.add(&result.Offset, &result.Offset, weight)
			result.Negate = true
		}
		result.SoftTerms[i] = term
	}
	if !o.ok() {
		return DecomposedObjective{}, nil
	}
	if result.ID, err = d.decompositionID(source, index); err != nil {
		return DecomposedObjective{}, err
	}
	return result, nil
}
