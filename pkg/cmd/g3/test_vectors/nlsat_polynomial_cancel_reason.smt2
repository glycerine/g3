(declare-const x Real)
(assert (> (+ (* x x x x) (* (- 10) x x) 1) 0))
(check-sat-using qfnra-nlsat)

(get-info :reason-unknown)
