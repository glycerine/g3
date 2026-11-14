(declare-const p0 Bool)
(declare-const x0 Int)
(assert (<= (div x0 0) (ite p0 2 x0)))
(apply qe2)
