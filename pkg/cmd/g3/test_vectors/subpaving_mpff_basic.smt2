(declare-const x Real)
(assert (<= x 1.0))
(check-sat-using (using-params subpaving :numeral mpff))
