; Reachable quantified-array proof provenance reproducer.
; The current top-down cone drops the fresh P(Int, Int) definition because P
; occurs below a forall in the Goal rule. Duality then reports sat, but the
; reconstructed hyper-resolution premise is not valid: a fresh SMT solver
; reports sat for its negation.
(set-option :fixedpoint.engine duality)
(set-option :fixedpoint.generate_proof_trace true)
(set-option :fixedpoint.xform.quantify_arrays true)
(declare-rel Seed ())
(declare-rel P ((Array Int Int)))
(declare-rel Goal ())
(rule Seed)
(rule (=> Seed (P ((as const (Array Int Int)) 7))))
(rule (=> (P ((as const (Array Int Int)) 7)) Goal))
(query Goal :print-answer false)
