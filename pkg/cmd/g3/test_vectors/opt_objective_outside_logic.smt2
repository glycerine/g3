(set-logic QF_LIA)(declare-const x Int)(declare-const y Int)(assert (< 0 x 3))(assert (< 0 y 3))(maximize (* x y))(check-sat)(get-objectives)
