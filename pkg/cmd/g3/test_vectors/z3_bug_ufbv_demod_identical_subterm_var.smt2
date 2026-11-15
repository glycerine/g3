; UFBV demodulator f(x, g(x)) = x + 1 must not rewrite f(c, g(y)) under
; forall y (AUDIT.md tactic_sls_ufbv_01-F1). y := d contradicts (= (f c (g d)) #x07),
; so this is unsat; z3 4.7.1 answers sat.
(set-logic UFBV)
(declare-fun f ((_ BitVec 8) (_ BitVec 8)) (_ BitVec 8))
(declare-fun g ((_ BitVec 8)) (_ BitVec 8))
(declare-const c (_ BitVec 8))
(declare-const d (_ BitVec 8))
(assert (forall ((x (_ BitVec 8))) (= (f x (g x)) (bvadd x #x01))))
(assert (forall ((y (_ BitVec 8))) (bvule (f c (g y)) #x05)))
(assert (= (f c (g d)) #x07))
(assert (= (f d (g c)) #x09))
(check-sat)
