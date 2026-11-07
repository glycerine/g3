package smtcore

// Source: src/smt/theory_arith_int.h. The integer procedures (gcd_test,
// Gomory cuts, branch and bound) are ported in the final-check phase of
// ARITH_PORT.md; until then gcd_test reports no conflict.

func (e *smtArithEngine) gcdTest(rowID int) (bool, error) { return true, nil }
