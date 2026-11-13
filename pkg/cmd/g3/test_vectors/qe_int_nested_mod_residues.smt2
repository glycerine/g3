(declare-const a Int)(assert (exists ((x Int)) (and (= (mod x 3) 1) (< a x) (< x (+ a 5)))))(apply qe)
