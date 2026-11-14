; With smt.relevancy 1, assign_eh reaches propagate_recognizer for a false
; recognizer that relevant_eh never registered (pkg/AUDIT.md smtcore_10-F3,
; upstream Z3 issue #4623, fixed by 7fc4653e47). z3 4.7.1 answers unsat;
; b = false is a model.
(set-option :smt.relevancy 1)
(declare-datatypes () ((L nil (cons (hd Int) (tl L)))))
(declare-const l L)
(declare-const b Bool)
(assert (or (not b) (is-cons l)))
(check-sat)
(get-value (b))
