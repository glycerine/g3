(declare-const a Int)
(declare-const b Int)
(assert (forall ((x Int) (y Int)) (or (not (= x a)) (not (= y b)))))
(apply der)
