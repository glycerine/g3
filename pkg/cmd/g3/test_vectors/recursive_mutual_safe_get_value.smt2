; Companion of recursive_mutual_boolean_model.smt2 that z3 4.7.1 can finish.
; z3 4.7.1 never terminates evaluating even/odd in the model: their
; preprocessed bodies are (or (<= n 0) (odd (+ -1 n))) and
; (and (not (<= n 0)) (even (+ -1 n))), and the model evaluator rewrites every
; or/and argument before reducing them (rewriter_tpl::constant_fold folds only
; ite early), so each unfolding continues into the dead branch forever. g3
; keeps its and/or short-circuit (a deliberate fix, see TODO.md "z3 4.7.1 bugs
; fixed in g3"), so this vector asks get-value only for terms both finish.
(set-option :produce-models true)
(set-logic AUFLIA)
(define-funs-rec ((even ((n Int)) Bool) (odd ((n Int)) Bool))
  ((ite (<= n 0) true (odd (- n 1)))
   (ite (<= n 0) false (even (- n 1)))))
(define-fun-rec choose ((flag Bool) (n Int)) Int (ite flag n (- n)))
(define-fun-rec zero () Int 0)
(define-fun-rec identity ((x Bool)) Bool x)
(assert (even 4))
(check-sat-using (using-params smt :auto_config false))
(get-value ((choose true 7) (choose false 7) zero (identity true) (identity false)))
(push 1)
(assert (odd 4))
(check-sat-using (using-params smt :auto_config false))
(pop 1)
(check-sat-using (using-params smt :auto_config false))
