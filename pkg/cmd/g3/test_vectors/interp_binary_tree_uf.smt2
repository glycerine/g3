(declare-fun f (Int) Int)
(declare-const a Int)
(declare-const b Int)
(compute-interpolant (interp (= (f a) 1)) (interp (= a b)) (not (= (f b) 1)))
