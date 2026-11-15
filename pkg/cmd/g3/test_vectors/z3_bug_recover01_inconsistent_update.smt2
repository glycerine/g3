; recover-01 rewrites (= p (not p)) to false and the goal collapses to
; false; z3 4.7.1 keeps indexing up to the old size and segfaults
; (AUDIT.md tactic_arith_03-F9).
(declare-const x Int)
(declare-const p Bool)
(declare-const b Bool)
(assert (= p (not p)))
(assert b)
(assert (> x 0))
(assert (or p (= x 0)))
(assert (or (not p) (= x 3)))
(apply recover-01)
