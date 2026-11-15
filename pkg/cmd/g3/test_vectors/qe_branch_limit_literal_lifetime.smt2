(declare-const a Real)
(declare-const b Real)
(assert (exists ((x Real) (y Real)) (and (< b y) (not (= a (ite (<= 7.0 y) b x))))))
(apply qe)
