(declare-const x0 Int)
(assert (= (+ 6 (div x0 x0)) (* 5 (- 10 x0 x0))))
(check-sat)
(get-model)
