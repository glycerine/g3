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
	// AUDIT.md tactic_arith_03-F10: purify-arith construction order (AST trace log
	// checked by TestTraceLogCPPReference).
	scriptResults["purify_root_power_order.smt2"] = "(goals\n(goal\n  (not (<= k!0 k!1))\n  (not (<= k!2 k!3))\n  (not (<= k!4 k!5))\n  (or (not (>= x 0.0)) (and (= x (^ k!0 2.0)) (>= k!0 0.0)))\n  (or (>= x 0.0) (= k!0 (neg-root x 2.0)))\n  (= y (^ k!1 3.0))\n  (or (= z 0.0) (= k!2 1.0))\n  (or (not (= z 0.0)) (= k!2 (^ 0.0 0.0)))\n  (or (not (>= x 0.0)) (and (= x (^ k!3 4.0)) (>= k!3 0.0)))\n  (or (>= x 0.0) (= k!3 (neg-root x 4.0)))\n  (or (not (>= x (- 1.0)))\n      (not (<= x 1.0))\n      (and (= x (sin k!4))\n           (>= k!4 (* (- (/ 1.0 2.0)) pi))\n           (<= k!4 (* (/ 1.0 2.0) pi))))\n  (or (>= x (- 1.0)) (= k!4 (asin-u x)))\n  (or (<= x 1.0) (= k!4 (asin-u x)))\n  (or (not (>= y (- 1.0)))\n      (not (<= y 1.0))\n      (and (= y (cos k!5)) (>= k!5 0.0) (<= k!5 pi)))\n  (or (>= y (- 1.0)) (= k!5 (acos-u y)))\n  (or (<= y 1.0) (= k!5 (acos-u y)))\n  :precision precise :depth 4)\n)\n(filter-model-converter k!4 k!0 k!2 k!3 k!5 k!1)\n"
	// AUDIT.md tactic_arith_03-F10: purify-arith construction order (AST trace log
	// checked by TestTraceLogCPPReference).
	scriptResults["purify_asin_proof_order.smt2"] = "(goals\n(goal\n  (not (<= k!0 k!1))\n  (or (not (>= x (- 1.0)))\n      (not (<= x 1.0))\n      (and (= x (sin k!0))\n           (>= k!0 (* (- (/ 1.0 2.0)) pi))\n           (<= k!0 (* (/ 1.0 2.0) pi))))\n  (or (>= x (- 1.0)) (= k!0 (asin-u x)))\n  (or (<= x 1.0) (= k!0 (asin-u x)))\n  (or (not (>= y (- 1.0)))\n      (not (<= y 1.0))\n      (and (= y (cos k!1)) (>= k!1 0.0) (<= k!1 pi)))\n  (or (>= y (- 1.0)) (= k!1 (acos-u y)))\n  (or (<= y 1.0) (= k!1 (acos-u y)))\n  :precision precise :depth 4)\n)\n"
	// AUDIT.md tactic_arith_03-F10: purify-arith construction order (AST trace log
	// checked by TestTraceLogCPPReference).
	scriptResults["purify_root_object_order.smt2"] = "(goals\n(goal\n  (not (<= y k!0))\n  (= (+ (* 2.0 k!0) (^ k!0 3.0)) 5.0)\n  (not (<= k!0 0.0))\n  (not (<= 4.0 k!0))\n  :precision precise :depth 4)\n)\n"
}
