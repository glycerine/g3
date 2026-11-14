(declare-const x0 Int)
(declare-const x1 Int)
(assert (not (<= (+ (* (- 1) x0) x1) (+ 10 (rem 2 x1) (* (- 1) (ite (>= x0 0) x0 (* (- 1) x0)))))))
(apply cofactor-term-ite)
