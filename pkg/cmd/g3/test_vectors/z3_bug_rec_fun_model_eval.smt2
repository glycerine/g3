; The model interprets even as (or (<= x 0) (odd (+ -1 x))). z3 4.7.1's
; model evaluator rewrites every or/and argument before reducing, so
; get-value of (even 0) unfolds the dead (odd -1) branch forever. The
; correct values are true, false, true.
(set-option :produce-models true)
(define-funs-rec ((even ((n Int)) Bool) (odd ((n Int)) Bool))
  ((ite (<= n 0) true (odd (- n 1)))
   (ite (<= n 0) false (even (- n 1)))))
(assert (even 4))
(check-sat-using (using-params smt :auto_config false))
(get-value ((even 0) (even 5) (odd 5)))
