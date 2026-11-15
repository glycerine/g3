(declare-const a Int)(declare-const b Int)
(assert (exists ((x Int)) (and (= (mod (+ x a) (- 3)) 1) (< x b) (> x a))))
(apply qe)
