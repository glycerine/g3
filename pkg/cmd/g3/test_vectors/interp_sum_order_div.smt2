(declare-const x Int)
(declare-const y Int)
(declare-const z Int)
(compute-interpolant (interp (and (= (* 2 x) (+ y 1)) (>= y 0))) (and (= y (* 2 z)) (<= z 10)))
