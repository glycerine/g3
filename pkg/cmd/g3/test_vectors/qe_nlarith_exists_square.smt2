(declare-const y Real)
(assert (exists ((x Real)) (= (* x x) y)))
(apply (using-params qe :qe_nonlinear true))
