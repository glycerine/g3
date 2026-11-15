package main

// z3BugExpectErrors lists the z3BugResults vectors whose correct output
// includes a command error (an "(error ...)" line with a line and column),
// so run reports an error at the end; TestZ3BugVectorsKeepCorrectAnswers
// still compares their whole output.
var z3BugExpectErrors = map[string]bool{
	"z3_bug_bvarray2uf_quantifier.smt2": true, // apply: not handled by bvarray2uf
	"z3_bug_get_value_max_steps.smt2":   true, // get-value: max. steps exceeded
}
