; purify-arith must not replace (sin x) when x also occurs outside sin/cos
; (AUDIT.md tactic_arith_02-F2). x = 10 and sin 10 /= 1/2, so this is unsat;
; z3 4.7.1 answers sat with an x that violates x/y = 1.
(set-option :produce-models true)
(declare-const x Real)
(declare-const y Real)
(assert (= (/ x y) 1.0))
(assert (= y 10.0))
(assert (= (sin x) 0.5))
(check-sat-using qfufnra)
(get-value (x y))
