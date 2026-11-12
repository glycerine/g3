; Reproduction of a z3 bug present in z3 4.7.1 and upstream z3 4.17.0
; (see TODO.md "z3 4.7.1 bugs fixed in g3").
; With i = j the outer store overwrites v1, so a = anything works and the
; problem is sat (plain (check-sat) says sat). Under (then qe smt), z3's
; qe_array_plugin solve_store emits (= (select (store b i v1) j) v2) and
; (= (select b i) v1), which forces v1 = v2, so z3 answers unsat.
; g3 answers sat.
(declare-const b (Array Int Int))
(declare-const i Int)
(declare-const j Int)
(declare-const v1 Int)
(declare-const v2 Int)
(assert (exists ((a (Array Int Int))) (= b (store (store a i v1) j v2))))
(assert (= i j))
(assert (= v1 0))
(assert (= v2 1))
(check-sat-using (then qe smt))
