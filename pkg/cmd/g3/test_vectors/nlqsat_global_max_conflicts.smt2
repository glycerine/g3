(set-option :nlsat.max_conflicts 1)
(declare-const x Real)
(assert (forall ((y Real)) (or (> (* (+ y 1.0) (+ y 2.0) (+ y 3.0)) 0.0) (< (* x y) 3.0) (= (* y y) (* 4.0 x)))))
(check-sat-using (then simplify nlqsat))
