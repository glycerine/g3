; lia2card on an equation between numerals (AUDIT.md tactic_arith_02, Needs
; verification): z3 4.7.1 builds (and false true), goal::update appends it
; unflattened and the rewrite loop never ends. The goal is false.
(assert (= 2 3))
(apply lia2card)
