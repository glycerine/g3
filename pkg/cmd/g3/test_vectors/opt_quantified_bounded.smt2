(declare-fun f (Int) Int)(declare-const x Int)(assert (forall ((y Int)) (> (f y) x)))(assert (< x 5))(maximize x)(check-sat)(get-objectives)
