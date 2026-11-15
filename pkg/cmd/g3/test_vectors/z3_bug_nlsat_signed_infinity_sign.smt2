(assert (forall ((a Real)) (exists ((x Real)) (and (< (* a x) (- 1.0)) (> (* (- a 2.0) x) 3.0)))))
(check-sat-using nlqsat)
