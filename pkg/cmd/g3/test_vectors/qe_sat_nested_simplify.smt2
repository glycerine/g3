(declare-fun a () Int)
(declare-fun b () Int)
(assert (exists ((x Int)) (and (> x a) (exists ((y Int)) (and (> y x) (< y b))))))
(apply qe-sat)
