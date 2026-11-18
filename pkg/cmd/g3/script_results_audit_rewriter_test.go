package main

// Expected z3 outputs for the AST rewriter audit vectors, kept apart from
// the scriptResults literal so that concurrent additions do not collide.
func init() {
	// AUDIT.md ast_rewriter_04-F8: pb2bv mk_le_ge uses the raw m.mk_not (so
	// (not (not a)) stays), dualize and is_pb use ast_util's ::mk_not.
	scriptResults["card2bv_negation_flavors.smt2"] = "(goals\n(goal\n  (not (not a))\n  :precision precise :depth 1)\n)\n(goals\n(goal\n  a\n  (let ((a!1 ((_ extract 2 2)\n               (bvadd (concat #b0 (ite b #b01 #b00))\n                      (concat #b0 (ite c #b01 #b00))))))\n    (= a!1 #b0))\n  true\n  :precision precise :depth 1)\n)\n(goals\n(goal\n  a\n  :precision precise :depth 1)\n)\n"
}
