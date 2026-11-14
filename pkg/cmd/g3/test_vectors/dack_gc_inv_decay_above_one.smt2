(set-option :smt.dack.gc_inv_decay 1.5)
(declare-const p Bool)
(assert p)
(check-sat)
