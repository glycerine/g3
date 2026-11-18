(declare-fun f ((Array Int Bool)) Int)
(assert (forall ((a (Array Int Bool))) (> (f a) 2)))
(assert (forall ((a (Array Int Bool)) (b (Array Int Bool))) (or (= a b) (not (= (f a) (f b))))))
(check-sat)
