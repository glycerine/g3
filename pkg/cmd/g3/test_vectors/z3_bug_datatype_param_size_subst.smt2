; Wrap's size function instantiates Box at Wrap's own parameter U. z3 4.7.1
; get_sort_size keys the substitution by U instead of Box's formal T, so
; sparam::subst misses and z3 segfaults. (Wrap Bool) has two values, so
; three distinct ones are unsat.
(declare-datatypes ((Box 1)) ((par (T) ((box (unbox T))))))
(declare-datatypes ((Wrap 1)) ((par (U) ((wrap (unwrap (Box U)))))))
(declare-const w (Wrap Bool))
(declare-const x (Wrap Bool))
(declare-const y (Wrap Bool))
(assert (distinct w x y))
(check-sat)
