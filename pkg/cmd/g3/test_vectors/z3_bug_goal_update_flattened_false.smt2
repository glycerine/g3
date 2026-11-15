; goal::update flattens nnf's (and false (not a)) to false but z3 4.7.1
; appends the unflattened formula, so the goal is not marked inconsistent
; and keeps the stale (not (=> false a)). The correct goal is false.
(declare-const a Bool)
(assert (not (=> false a)))
(apply nnf)
