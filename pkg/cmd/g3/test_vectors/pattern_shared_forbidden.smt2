(declare-fun h (Int) Bool)
(declare-fun k (Bool Bool Bool) Bool)
(assert (forall ((x Int)) (! (k (not (and (h x) (h 1))) (not (and (h x) (h 1))) (or (h x) (h 2))) :pattern ((k (not (and (h x) (h 1))) (not (and (h x) (h 1))) (or (h x) (h 2)))))))
(check-sat)
