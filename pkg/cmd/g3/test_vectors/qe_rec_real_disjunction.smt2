(declare-const a Real)(declare-const b Real)(assert (exists ((x Real)) (and (< a x) (< (* 2 x) b) (or (> x 5.0) (< (+ x b) 7.0)))))(apply qe_rec)
