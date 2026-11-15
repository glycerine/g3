(declare-fun f (Int) Int)
(declare-fun g (Int) Int)
(declare-const b Int)
(compute-interpolant (interp (and (forall ((x Int)) (! (> (f x) (g x)) :pattern ((f x)))) (forall ((y Int)) (! (> (g y) 0) :pattern ((g y)))))) (< (f b) 0))
