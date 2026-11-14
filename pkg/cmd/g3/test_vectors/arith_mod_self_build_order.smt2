(declare-const x Int)
(declare-const y Int)
(assert (> (mod (+ x y) (+ x y)) 2))
(apply simplify)
