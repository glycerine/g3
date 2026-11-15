(declare-fun f (Int) Int)
(declare-const a Int)
(simplify (forall ((x Int)) (! (> (f a) 0) :pattern ((f x)))) :ignore_patterns_on_ground_qbody false)
