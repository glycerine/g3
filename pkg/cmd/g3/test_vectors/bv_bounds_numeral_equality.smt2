(declare-const p Bool)
(declare-const q Bool)
(assert (or p (ite (= #x01 #x02) (= #x03 #x02) q)))
(apply propagate-bv-bounds)
