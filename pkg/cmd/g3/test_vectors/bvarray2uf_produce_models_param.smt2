(declare-const a (Array (_ BitVec 4) (_ BitVec 4)))
(assert (= (select a #x1) #x2))
(apply (using-params bvarray2uf :produce_models true))
