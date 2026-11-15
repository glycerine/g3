(declare-const a Int)
(declare-const b Int)
(assert (exists ((x Int)) (and (= (mod (+ x a) 0) 0) (< x b))))
(apply qe)
