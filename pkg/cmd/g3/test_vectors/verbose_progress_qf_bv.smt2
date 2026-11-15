(set-logic QF_BV)
(declare-const x (_ BitVec 8))
(assert (= (bvmul x #x03) #x07))
(check-sat)
