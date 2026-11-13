(declare-const a Real)(declare-const b Real)(assert (forall ((x Real)) (exists ((y Real)) (and (< (+ x a) y) (or (< y (* 3 b)) (> (- y x) 2.0))))))(apply qe_rec)
