(declare-const s String)
(assert (= (str.len s) 3))
(check-sat)
