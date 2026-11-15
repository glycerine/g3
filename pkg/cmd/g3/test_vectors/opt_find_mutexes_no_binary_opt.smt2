; AUDIT.md smtcore_02-F10 follow-up (find_mutexes): with proofs the binary
; clause optimization is off, so the pairwise exclusions stay clause objects
; and context::find_mutexes, which reads only binary watch literals, finds no
; mutex. maxres then needs three cores instead of one mutex step.
(set-option :produce-proofs true)
(set-option :opt.enable_sat false)
(declare-const a Bool)
(declare-const b Bool)
(declare-const c Bool)
(assert (or (not a) (not b)))
(assert (or (not b) (not c)))
(assert (or (not a) (not c)))
(assert-soft a :weight 3)
(assert-soft b :weight 2)
(assert-soft c :weight 1)
(check-sat)
(get-objectives)
(get-model)
