; AUDIT.md ast_05-F2: (_ to_fp eb sb) of one Real is not a supported
; combination (upstream z3; the fork no longer reads past the domain).
(declare-const f (_ FloatingPoint 3 3))
(declare-const r Real)
(assert (= f ((_ to_fp 3 3) 1.5)))
(assert (= f ((_ to_fp 3 3) r)))
(assert (= f ((_ to_fp 3 3) RNE r)))
(check-sat)
(simplify ((_ to_fp 3 3) 1.0))
(simplify ((_ to_fp 3 3) RNE 1.5))
