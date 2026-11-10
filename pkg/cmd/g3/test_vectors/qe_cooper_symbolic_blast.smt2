(declare-const y Int)
(declare-const z Int)
(assert (exists ((x Int)) (and (<= (* 5 x) y) (>= (* 7 x) z))))
(apply qe)
