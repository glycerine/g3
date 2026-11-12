; z3 4.7.1 bug (opt_context::from_fmls stale orig_term): z3 4.7.1 prints
; (q (- 1)); the optimum, printed by g3 and z3 4.17.0, is (q (- 3))
; (a true, b false). Excluded from z3 identity testing; see
; z3_bug_exclusions.txt and TODO.md "z3 4.7.1 bugs fixed in g3".
(set-option :opt.priority box)
(declare-const a Bool)
(declare-const b Bool)
(declare-const x Int)
(declare-const y Int)
(assert (<= x 7))
(assert (=> a (< x 3)))
(assert (=> b (> x 1)))
(maximize x)
(maximize y)
(assert-soft a :weight 2 :id q)
(assert-soft b :weight -3 :id q)
(check-sat)
(get-objectives)
