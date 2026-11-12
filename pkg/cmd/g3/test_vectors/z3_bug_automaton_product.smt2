; Reproduction of a z3 4.7.1 bug (see TODO.md "z3 4.7.1 bugs fixed in g3").
; x = "" is in ("" | a) & ("" | b), so the correct answer is sat.
; z3 4.7.1 answers unsat: symbolic_automata::mk_product tests
; is_final_state without the epsilon closure, so the product is empty.
; g3 answers sat with x = "".
(declare-const x String)
(assert (str.in.re x (re.inter (re.union (str.to.re "") (str.to.re "a")) (re.union (str.to.re "") (str.to.re "b")))))
(check-sat)
(get-model)
