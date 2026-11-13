(declare-const a Int)(assert (exists ((x Int)) (forall ((y Int)) (or (< y x) (> y (+ x a))))))(check-sat-using qsat)
