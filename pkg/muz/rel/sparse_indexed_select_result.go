package rel

func (plan *SparseIndexedSelectPlan) newResult(table *SparseTable) (*SparseTable, error) {
	if plan == nil {
		return nil, ErrTable
	}
	return table.plugin.MkEmpty(plan.resultSignature, plan.resultFunctional)
}

func (plan *SparseIndexedSelectPlan) ResultSignature() (Signature, int, error) {
	if plan == nil {
		return Signature{}, 0, ErrTable
	}
	return Signature{domains: plan.resultSignature.Domains()}, plan.resultFunctional, nil
}
