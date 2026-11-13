(declare-const a Real)(assert (exists ((x Real)) (forall ((y Real)) (or (< y x) (> y (+ x a))))))(check-sat-using qsat)
