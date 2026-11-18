(declare-const y Real)
(assert (> y (root-obj (+ (^ x 3) (* 2 x) (- 5)) 1)))
(apply purify-arith)
