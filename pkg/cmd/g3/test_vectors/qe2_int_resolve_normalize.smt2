(declare-const y Int)
(declare-const z Int)
(assert (exists ((x Int)) (and (<= x (* 2 y)) (<= (* 2 z) x))))
(apply qe2)
