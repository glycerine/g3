//go:build g3checked || g3debug

package polynomial

// polynomialValidateOwnership: see context_validate_off.go. Checked and debug
// builds run every ownership/identity test of the allocation-context
// validators.
const polynomialValidateOwnership = true
