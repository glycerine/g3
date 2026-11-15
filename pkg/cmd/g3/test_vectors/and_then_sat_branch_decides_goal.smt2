; AUDIT.md tactic_top_01-F1: split-clause yields an OR-decomposition; the
; second subgoal is decided sat by solve-eqs, so the goal is sat with that
; branch's model even though the first subgoal stays undecided.
(set-option :produce-models true)
(declare-const x Int)
(declare-const y Int)
(assert (or (> x 0) (= y 3)))
(apply (then split-clause solve-eqs))
(check-sat-using (then split-clause solve-eqs))
(get-model)
(eval (or (> x 0) (= y 3)))
(check-sat-using (par-then split-clause solve-eqs))
(get-model)
(eval (or (> x 0) (= y 3)))
