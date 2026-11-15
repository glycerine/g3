(declare-const a (Array Int Int))
(assert (= (select a 1) 2))
(check-sat)
