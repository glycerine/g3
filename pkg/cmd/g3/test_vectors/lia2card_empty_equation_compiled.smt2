; With compile_equality, lia2card turns (= 2 3) and (= 2 2) into pbeq atoms
; without terms; z3 does not fold them (companion of
; z3_bug_lia2card_empty_equation).
(push)
(assert (= 2 3))
(apply (using-params lia2card :compile_equality true))
(pop)
(assert (= 2 2))
(apply (using-params lia2card :compile_equality true))
