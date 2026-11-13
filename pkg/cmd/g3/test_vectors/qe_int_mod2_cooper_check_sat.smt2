(declare-const a Int)(declare-const b Int)(assert (exists ((x Int)) (and (= (mod x 2) 0) (<= (* 3 x) b) (>= (* 5 x) a))))(check-sat-using qe)
