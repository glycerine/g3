(declare-const a Int)(declare-const b Int)(assert (forall ((x Int)) (=> (> x a) (exists ((y Int)) (and (< x y) (< (* 2 y) (+ x b)))))))(apply qe_rec)
