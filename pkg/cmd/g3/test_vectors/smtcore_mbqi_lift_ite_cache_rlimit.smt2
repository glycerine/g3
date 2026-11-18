; AUDIT.md smtcore_05-F4: asserted_formulas owns its lift-ite rewriters, so
; their caches persist across the model checker's auxiliary-context rounds.
; z3 spends 14196 resource units here; a fresh rewriter per round spent 14228.
(set-option :rlimit 14200)
(declare-fun f ((_ BitVec 8)) (_ BitVec 8))
(assert (forall ((x (_ BitVec 8))) (bvuge (f (bvadd x #x01)) (f x))))
(assert (= (f #x05) #x03))
(assert (= (f #x09) #x01))
(check-sat)
