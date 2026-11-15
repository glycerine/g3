(set-option :rlimit 2000)
(declare-const a Int)
(assert (forall ((y Int)) (or (< y 0) (exists ((x Int)) (and (>= (* 4 x) y) (<= (* 6 x) (+ y a)) (= (mod x 3) 1))))))
(check-sat-using (using-params qe-sat :projection_mode false))

(get-info :reason-unknown)
