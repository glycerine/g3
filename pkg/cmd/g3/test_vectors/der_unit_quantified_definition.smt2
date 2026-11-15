(declare-const b Bool)
(assert (forall ((x Bool)) (not (= x (forall ((y Int)) (> y 0))))))
(apply der)
