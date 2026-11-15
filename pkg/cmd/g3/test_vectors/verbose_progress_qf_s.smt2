(set-logic QF_S)
(declare-const s String)
(assert (= (str.++ s "a") "ba"))
(check-sat)
