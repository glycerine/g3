(declare-fun a () Int)
(declare-fun b () Int)
(assert (forall ((x Int)) (exists ((y Int)) (and (or (> x a) (< y x)) (> (+ y b) x)))))
(apply qe-sat)
