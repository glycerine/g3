(declare-const y Int)
(declare-const p Bool)
(assert (or p (forall ((x Int)) (not (= x y)))))
(check-sat-using qe-sat)
