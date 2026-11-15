(declare-fun f (Int) Int)
(assert (forall ((x Int)) (= (f x) 3)))
(assert (> (f 3) 1))
(check-sat)
