(declare-fun p (Int) Bool)
(declare-fun z () Bool)
(assert (forall ((x Int) (b Bool)) (=> (not (p x)) b)))
(check-sat-using horn)
