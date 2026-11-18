package opt

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"

	"github.com/glycerine/g3/pkg/ast"
)

func (d *ObjectiveDecomposer) convertBitVector(source DecodedFormulaObjective, index int) (DecomposedObjective, bool, error) {
	isBV, err := d.manager.IsBitVector(source.Term)
	if err != nil || !isBV {
		return DecomposedObjective{}, false, err
	}
	width, err := d.manager.BitVectorSize(source.Term)
	if err != nil {
		return DecomposedObjective{}, false, err
	}
	result := DecomposedObjective{SourceKind: source.Kind, OriginalTerm: source.OriginalTerm, Converted: true, Negate: source.Kind == FormulaMaximize}
	result.SoftTerms = make([]ast.ASTHandle, 0, width)
	result.Weights = make([]numeral.Rational, 0, width)
	result.Offset = numeral.NewRational()
	o := newRatOps(d.manager)
	fail := func(cause error) (DecomposedObjective, bool, error) {
		return DecomposedObjective{}, false, errors.Join(cause, o.err, result.ReleaseRationals(o.q))
	}
	bitValue := uint32(0)
	if source.Kind == FormulaMaximize {
		bitValue = 1
	}
	value, err := d.manager.BitVectorNumeralUnsigned(bitValue, 1)
	if err != nil {
		return fail(err)
	}
	// rational weight(1), doubled per bit
	weight, two := o.int64(1), o.int64(2)
	defer o.del(&two)
	defer o.del(&weight)
	for bit := uint32(0); bit < width; bit++ {
		extract, e := d.manager.MkBitVectorExtract(bit, bit, source.Term)
		if e != nil {
			return fail(e)
		}
		extract, e = d.retain(extract)
		if e != nil {
			return fail(e)
		}
		eq, e := d.manager.MkEq(value, extract)
		if e != nil {
			return fail(e)
		}
		eq, e = d.retain(eq)
		if e != nil {
			return fail(e)
		}
		result.SoftTerms = append(result.SoftTerms, eq)
		result.Weights = append(result.Weights, o.clone(&weight))
		if source.Kind == FormulaMaximize {
			o.add(&result.Offset, &result.Offset, &weight)
		}
		o.mul(&weight, &weight, &two)
		if !o.ok() {
			return fail(nil)
		}
	}
	if result.ID, err = d.decompositionID(source, index); err != nil {
		return fail(err)
	}
	return result, true, nil
}
