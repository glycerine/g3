(declare-const x Real)
(assert (> (- (* x x) 2) 0))
(check-sat-using qfnra-nlsat)
