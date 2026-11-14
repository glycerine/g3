(declare-const a (Array (_ BitVec 3) (_ BitVec 3)))
(declare-const b (Array (_ BitVec 3) (_ BitVec 3)))
(assert (distinct a b))
(apply bvarray2uf)
