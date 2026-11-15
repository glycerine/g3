; card2bv on a Real 0-1 sum with fractional coefficients (AUDIT.md
; ast_rewriter_04-F3): z3 4.7.1 builds an always-true circuit and answers
; sat; the constraint must be left unchanged and the problem is unsat.
(declare-const a Bool)
(declare-const b Bool)
(assert (<= (+ (ite a 0.5 0.0) (ite b 0.5 0.0)) 0.7))
(apply card2bv)
(assert a)
(assert b)
(check-sat-using (then card2bv smt))
