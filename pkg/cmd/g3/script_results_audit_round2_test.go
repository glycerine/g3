package main

// Expected z3 outputs for the second-round audit vectors, kept apart from
// the scriptResults literal so that concurrent additions do not collide.
func init() {
	// theory_array::pop_scope_eh pops m_trail_stack (the m_sort2epsilon
	// ast2ast_trail) before restore_sorts releases the extensionality
	// skolems (AST event order checked by TestASTEventOrder).
	scriptResults["array_epsilon_pop_order.smt2"] = "unknown\n"
	// tseitin_cnf_tactic's imp keeps a failed run's clauses and auxiliary
	// variables until cleanup() (AST event order checked by
	// TestASTEventOrder, the trace=true log by TestTraceLogCPPReference).
	scriptResults["tseitin_core_failure_state.smt2"] = "(goals\n(goal\n  (or (not k!4) a (<= x 2))\n  (or (not k!4) (not a) (not (<= x 2)))\n  (or k!4 (not a) (<= x 2))\n  (or k!4 a (not (<= x 2)))\n  (or (not c) a (not k!5))\n  (or c b (not k!5))\n  (or (not c) (not a) k!5)\n  (or c (not b) k!5)\n  (or a b (not k!5))\n  (or (not a) (not b) k!5)\n  (or k!5 d k!4)\n  (or (not c) k!6)\n  (or (not d) k!6)\n  (or c d (not k!6))\n  (or (not k!7) b (not k!6))\n  (or (not k!7) (not b) k!6)\n  (or k!7 (not b) (not k!6))\n  (or k!7 b k!6)\n  (or (not a) k!7)\n  (or a b)\n  (or (not (<= x 2)) b)\n  (or a c)\n  (or (not (<= x 2)) c)\n  :precision precise :depth 2)\n)\n(goals\n(goal\n  (or (not k!12) a (<= x 2))\n  (or (not k!12) (not a) (not (<= x 2)))\n  (or k!12 (not a) (<= x 2))\n  (or k!12 a (not (<= x 2)))\n  (or (not c) a (not k!13))\n  (or c b (not k!13))\n  (or (not c) (not a) k!13)\n  (or c (not b) k!13)\n  (or a b (not k!13))\n  (or (not a) (not b) k!13)\n  (or k!13 d k!12)\n  (or (not c) k!14)\n  (or (not d) k!14)\n  (or c d (not k!14))\n  (or (not k!15) b (not k!14))\n  (or (not k!15) (not b) k!14)\n  (or k!15 (not b) (not k!14))\n  (or k!15 b k!14)\n  (or (not a) k!15)\n  (or a b)\n  (or (not (<= x 2)) b)\n  (or a c)\n  (or (not (<= x 2)) c)\n  (or (not a) (not (<= x 1)) (not k!16))\n  (or a c (not k!16))\n  (or (not a) (<= x 1) k!16)\n  (or a (not c) k!16)\n  (or (not (<= x 1)) c (not k!16))\n  (or (<= x 1) (not c) k!16)\n  (or e k!16)\n  :precision precise :depth 2)\n)\n"
}
