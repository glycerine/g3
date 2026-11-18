package main

// Expected z3 outputs for the second-round audit vectors, kept apart from
// the scriptResults literal so that concurrent additions do not collide.
func init() {
	// theory_array::pop_scope_eh pops m_trail_stack (the m_sort2epsilon
	// ast2ast_trail) before restore_sorts releases the extensionality
	// skolems (AST event order checked by TestASTEventOrder).
	scriptResults["array_epsilon_pop_order.smt2"] = "unknown\n"
}
