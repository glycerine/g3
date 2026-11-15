(declare-const y Real)
(declare-const z Real)
(assert (exists ((x Real)) (and (= (+ (* x x) (* z x)) y) (< x 1.0))))
(apply (using-params qe :qe_nonlinear true))
