; xform.karr: mk_karr_invariants inner datalog context rejects the
; loop-counter rules (Int columns); the query fails and the rule set the
; transformer held is freed while unwinding (fixed fork, FORK FIX #112).
(set-option :fixedpoint.engine spacer)
(set-option :fixedpoint.xform.karr true)
(declare-rel p (Int))
(declare-rel q ())
(declare-var x Int)
(rule (p 0))
(rule (=> (and (p x) (< x 10)) (p (+ x 1))))
(rule (=> (and (p x) (> x 20)) q))
(query q)
