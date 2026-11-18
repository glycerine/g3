package main

// Expected z3 outputs for the AST rewriter audit vectors, kept apart from
// the scriptResults literal so that concurrent additions do not collide.
func init() {
	// AUDIT.md ast_rewriter_04-F8: pb2bv mk_le_ge uses the raw m.mk_not (so
	// (not (not a)) stays), dualize and is_pb use ast_util's ::mk_not.
	scriptResults["card2bv_negation_flavors.smt2"] = "(goals\n(goal\n  (not (not a))\n  :precision precise :depth 1)\n)\n(goals\n(goal\n  a\n  (let ((a!1 ((_ extract 2 2)\n               (bvadd (concat #b0 (ite b #b01 #b00))\n                      (concat #b0 (ite c #b01 #b00))))))\n    (= a!1 #b0))\n  true\n  :precision precise :depth 1)\n)\n(goals\n(goal\n  a\n  :precision precise :depth 1)\n)\n"
	// AUDIT.md ast_rewriter_05-F6: mk_seq_index folds two literals only for an
	// r.is_unsigned() offset.
	scriptResults["seq_unsigned_bounds.smt2"] = "(str.indexof \"abc\" \"b\" 4294967296)\n(- 1)\n(str.indexof \"abc\" \"\" 4294967296)\n(- 1)\n2\n"
	// AUDIT.md ast_rewriter_05-F6, fork fix (re_loop_int_bound): z3 4.7.1
	// folded a loop bound of 2^31 or more into a negative int parameter.
	scriptResults["z3_bug_re_loop_int_bound.smt2"] = "(re.loop (str.to.re \"a\") 2147483648)\n(re.loop (str.to.re \"a\") 1 2147483648)\n((_ re.loop 2147483647) (str.to.re \"a\"))\n"
}
