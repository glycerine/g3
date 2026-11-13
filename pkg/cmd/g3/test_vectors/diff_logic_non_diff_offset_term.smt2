; theory_diff_logic::mk_var(app*) flags the interpreted offset term
; (+ -1/4 r) when arith_eq_adapter internalizes (<= (+ -1/4 r) 5/6); the
; final check gives up, and found_non_diff_logic_expr reports the term on
; the verbose stream (stderr). A get-model here would report "model is not
; available", as m_gen2_5298 does; run() returns that as an error.
(set-option :produce-models true)
(declare-const a Bool)
(declare-const r Real)
(assert (or (not a) (distinct (/ 5.0 6.0) 1.0 (- r (/ 1.0 4.0)))))
(check-sat)
(get-info :reason-unknown)
(push 1)
(assert (= r (/ 1.0 2.0)))
(check-sat)
(pop 1)
(check-sat)
