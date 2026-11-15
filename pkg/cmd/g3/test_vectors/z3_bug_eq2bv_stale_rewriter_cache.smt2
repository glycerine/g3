; eq2bv keeps its rewriter cache across goals (AUDIT.md tactic_arith_01-F3).
; z3 4.7.1 reuses the first goal's (= z!0 #b11) in the second goal and says sat.
(declare-const x Int)
(declare-const p Bool)
(declare-const b Bool)
(declare-const c Bool)
(assert (or p (not p)))
(assert (<= x 2))
(assert (or (= x 3) b))
(assert (or (= x 3) c))
(assert (not b))
(check-sat-using (then split-clause eq2bv smt))
