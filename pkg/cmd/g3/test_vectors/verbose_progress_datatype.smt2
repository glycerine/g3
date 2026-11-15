(declare-datatypes ((L 0)) (((nil) (cons (hd Int) (tl L)))))
(declare-const l L)
(assert (is-cons l))
(check-sat)
