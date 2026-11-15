; AUDIT.md opt_01-F1: maxsat_engine=sortmax builds sortmax whatever the
; weight total. These weights expand to 4098 network inputs, above the
; 4096 at which g3 used to switch to MaxRes.
(set-option :opt.maxsat_engine sortmax)
(declare-const x Bool)
(declare-const y Bool)
(declare-const z Bool)
(assert (or y z))
(assert (or (not x) y z))
(assert-soft (not x) :weight 4093)
(assert-soft (not y) :weight 5)
(assert-soft (not z) :weight 3)
(check-sat)
(get-objectives)
(get-model)
