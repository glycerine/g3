(declare-const a Int)(declare-const b Int)(assert (forall ((x Int)) (exists ((y Int)) (and (<= (+ x a) (* 3 y)) (or (< y b) (> (- y x) 2))))))(apply qe)
