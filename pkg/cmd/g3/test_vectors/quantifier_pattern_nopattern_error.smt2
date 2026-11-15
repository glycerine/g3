(declare-fun p (Int) Bool)
(declare-fun q (Int) Bool)
(assert (forall ((x Int)) (! (p x) :pattern ((p x)) :no-pattern (q x))))
(check-sat)
