(declare-const a Int)(declare-const b Int)(assert (exists ((x Int)) (and (= (mod x 3) 1) (< a x) (< x b))))(apply qe)(check-sat)
