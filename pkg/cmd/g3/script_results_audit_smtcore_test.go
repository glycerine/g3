package main

// Expected z3 4.7.1 outputs for the smtcore audit vectors.

func init() {
	// AUDIT.md smtcore_05-F4: the lift_ite, ng_lift_ite,
	// cheap_quant_fourier_motzkin and pull_nested_quantifiers rewriters live
	// as long as asserted_formulas; their caches persist across reduce().
	scriptResults["smtcore_mbqi_lift_ite_cache_rlimit.smt2"] = "unsat\n"
}
