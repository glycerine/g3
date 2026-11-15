(declare-const y Int)
(assert (exists ((x Int) (z Int)) (and (= x y) (> z x))))
(apply qe-light)
