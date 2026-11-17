package rel

import "errors"

func checkedPairMixedJoin(first, second FiniteTable, left, right []int) (result FiniteTable, resultErr error) {
	closeLifted := func(lifted *FiniteCheckedPair) {
		closeErr := lifted.Close()
		if closeErr != nil && result != nil {
			closeErr = errors.Join(closeErr, result.Close())
			result = nil
		}
		resultErr = errors.Join(resultErr, closeErr)
	}
	if pair, ok := first.(*FiniteCheckedPair); ok {
		lifted, err := pair.Lift(second)
		if err != nil {
			return nil, err
		}
		defer func() { closeLifted(lifted) }()
		result, resultErr = pair.Join(lifted, left, right)
		return result, resultErr
	}
	pair, ok := second.(*FiniteCheckedPair)
	if !ok {
		return nil, ErrCheckTable
	}
	lifted, err := pair.Lift(first)
	if err != nil {
		return nil, err
	}
	defer func() { closeLifted(lifted) }()
	result, resultErr = lifted.Join(pair, left, right)
	return result, resultErr
}

func checkedPairMixedJoinProject(first, second FiniteTable, left, right, removed []int) (result FiniteTable, resultErr error) {
	closeLifted := func(lifted *FiniteCheckedPair) {
		closeErr := lifted.Close()
		if closeErr != nil && result != nil {
			closeErr = errors.Join(closeErr, result.Close())
			result = nil
		}
		resultErr = errors.Join(resultErr, closeErr)
	}
	if pair, ok := first.(*FiniteCheckedPair); ok {
		lifted, err := pair.Lift(second)
		if err != nil {
			return nil, err
		}
		defer func() { closeLifted(lifted) }()
		result, resultErr = pair.JoinProject(lifted, left, right, removed)
		return result, resultErr
	}
	pair, ok := second.(*FiniteCheckedPair)
	if !ok {
		return nil, ErrCheckTable
	}
	lifted, err := pair.Lift(first)
	if err != nil {
		return nil, err
	}
	defer func() { closeLifted(lifted) }()
	result, resultErr = lifted.JoinProject(pair, left, right, removed)
	return result, resultErr
}

func checkedPairMixedUnion(target *FiniteCheckedPair, source FiniteTable, delta *FiniteCheckedPair) (resultErr error) {
	lifted, err := target.Lift(source)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, lifted.Close()) }()
	return target.UnionFrom(lifted, delta)
}

func checkedPairMixedNegation(target *FiniteCheckedPair, negative FiniteTable, targetColumns, negativeColumns []int) (resultErr error) {
	lifted, err := target.Lift(negative)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, lifted.Close()) }()
	return target.FilterByNegation(lifted, targetColumns, negativeColumns)
}

// checkedPairOperandResult is default_table_join_fn's mk_empty of the
// result by the plugin of operand t (not a check table), when that plugin
// handles the result signature.
func checkedPairOperandResult(t FiniteTable, signature Signature, functional int) (FiniteTable, bool, error) {
	switch concrete := t.(type) {
	case *SparseTable:
		if signature.Len() != 0 {
			result, err := concrete.plugin.MkEmpty(signature, functional)
			return result, true, err
		}
	case *LazySparseTable:
		if signature.Len() != 0 {
			result, err := lazySparseInstructionResult(concrete, signature, functional)
			return result, true, err
		}
	case *HashTable:
		if functional == 0 {
			return NewHashTable(signature), true, nil
		}
	case *BitvectorTable:
		if BitvectorCanHandleSignature(signature, functional) {
			result, err := concrete.emptyLike(signature)
			return result, true, err
		}
	}
	return nil, false, nil
}

// checkedPairAppropriateTable is get_appropriate_plugin(sig).mk_empty(sig)
// with pair's check_table plugin the favourite: it, else the first
// registered plugin that handles the signature (sparse for a nonnullary
// one, hashtable for one without functional columns).
func checkedPairAppropriateTable(pair *FiniteCheckedPair, signature Signature, functional int) (FiniteTable, error) {
	if pair.plugin.CanHandleSignatureFunctional(signature, functional) {
		return pair.plugin.NewFunctional(signature, functional)
	}
	if signature.Len() != 0 {
		var plugin *SparseTablePlugin
		if pair.plugin != nil && pair.plugin.manager != nil {
			plugin = pair.plugin.manager.sparsePlugin()
		}
		return plugin.MkEmpty(signature, functional)
	}
	if functional == 0 {
		return NewHashTable(signature), nil
	}
	return nil, ErrCheckTable
}

// scopePairOperand reports a checked scope's pair (one without a
// check_table plugin) among the operands of a mixed operation.
func scopePairOperand(first, second FiniteTable) bool {
	if pair, ok := first.(*FiniteCheckedPair); ok && pair.plugin == nil {
		return true
	}
	pair, ok := second.(*FiniteCheckedPair)
	return ok && pair.plugin == nil
}
