//go:build g3checked || g3debug

package numeral

// numericValidateCells: see numeric_validate_off.go. Checked and debug builds
// re-validate a numeral cell's pooled object on every digit access.
const numericValidateCells = true
