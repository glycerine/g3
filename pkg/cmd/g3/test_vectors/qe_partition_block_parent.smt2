(declare-const a Int)
(declare-const b Int)
(assert (exists ((x Int) (y Int)) (and (> (* 2 x) a) (< (* 3 x) (+ a 7)) (< (* 3 y) b) (> (* 5 y) (- b 4)))))
(apply qe)
