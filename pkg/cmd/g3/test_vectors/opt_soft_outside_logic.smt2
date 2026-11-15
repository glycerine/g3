(set-logic QF_LIA)(declare-const x Int)(declare-const y Int)(assert (< 0 x 3))(assert (< 0 y 3))(assert-soft (= (* x y) 4))(check-sat)(get-objectives)
