package rel

// PrepareSourceEmptyBasis handles the native Karr dualizeI boundary for a
// relation with no inequality rows. The native Hilbert basis has zero
// variables there, so saturation produces no generators. It is used by the
// Datalog unbound-column consumer at the source get_basis call sites; other
// consumers retain their existing full-relation treatment.
func (r *KarrRelation) PrepareSourceEmptyBasis() {
	if r == nil || r.empty || r.basisValid || !r.inequalitiesValid || r.inequalities.Len() != 0 {
		return
	}
	// Native saturate checks the resource limit at loop entry and again on exit.
	if r.checkpoint != nil {
		r.checkpoint()
		r.checkpoint()
	}
	r.basis = KarrMatrix{}
	r.basisValid = true
}
