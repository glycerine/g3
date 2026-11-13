(declare-const a Real)(declare-const b Real)(assert (exists ((x Real)) (and (<= a x) (<= x b) (not (= x 3.0)))))(apply qe)
