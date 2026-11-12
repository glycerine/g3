package smtcore

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *smtImpliedEqualityEngine) modelPartition(indices []uint32, evaluator smtImpliedEqualityEvaluator) (result error) {
	buckets := make(map[ast.ASTHandle][]uint32)
	var bucketOrder []ast.ASTHandle // first-appearance order
	nonValues := make([]uint32, 0)
	refs := make([]ast.ASTRef, 0, len(indices))
	defer func() {
		for i := range refs {
			result = errors.Join(result, e.manager.DelRef(&refs[i]))
		}
	}()
	for _, index := range indices {
		ref, err := evaluator.EvaluateValue(e.terms[index])
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		value, err := e.manager.RefGet(ref)
		if err != nil {
			return err
		}
		isValue, err := e.manager.IsValue(value)
		if err != nil {
			return err
		}
		e.stats.ValueEvaluations++
		if !isValue {
			nonValues = append(nonValues, index)
			continue
		}
		if _, seen := buckets[value]; !seen {
			bucketOrder = append(bucketOrder, value)
		}
		buckets[value] = append(buckets[value], index)
	}
	e.stats.ValueBuckets += uint32(len(buckets))
	for _, value := range bucketOrder {
		if err := e.exact(buckets[value]); err != nil {
			return err
		}
	}
	if len(nonValues) != 0 {
		e.stats.FallbackTerms += uint32(len(nonValues))
		return e.exact(indices)
	}
	return nil
}
