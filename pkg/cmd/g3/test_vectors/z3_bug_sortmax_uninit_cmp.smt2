; sortmax builds its sorting network with psort_nw, whose constructor leaves
; m_t uninitialized in z3 4.7.1. When it reads LE the network outputs do not
; imply their inputs, and z3 reports objective 4 with a model of cost 11.
; The optimum is 5 (fork fix #A initializes m_t to GE, as upstream does).
(set-option :opt.maxsat_engine sortmax)
(declare-const a Bool)
(declare-const b Bool)
(declare-const c Bool)
(declare-const d Bool)
(assert (or (not a) (not b)))
(assert (or (not c) (not d)))
(assert (or (not a) (not d)))
(assert-soft a :weight 3)
(assert-soft b :weight 2)
(assert-soft c :weight 2)
(assert-soft d :weight 4)
(check-sat)
(get-objectives)
(get-model)
