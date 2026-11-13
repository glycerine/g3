(declare-const a Real)(assert (or (> a 3.0) (exists ((x Real)) (and (< a x) (forall ((y Real)) (or (< y x) (> y (+ a 1.0))))))))(apply qe_rec)
