package rel

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
