(declare-const a Int)
(assert (forall ((y Int)) (or (< y 0) (exists ((x Int)) (and (> (* 7 x) y) (< (* 5 x) (+ y 3)))))))
(check-sat-using (using-params qe-sat :projection_mode false))
