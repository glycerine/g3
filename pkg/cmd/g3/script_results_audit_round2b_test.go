package main

// Expected z3 4.7.1 outputs (with the fork's fixes) for the second round of
// cmd/api/interp/parser audit leftovers.

func init() {
	// A par binder longer than its (name arity) header crashes z3 4.7.1;
	// the fork reports the mismatch (FORK FIX #136, tst_z3_4_7_1_datatype_par_beyond_header).
	scriptResults["datatype_par_beyond_header.smt2"] = "(error \"line 1 column 39: datatype sort D declares arity 0 but defines 1 parameters\")\n(error \"line 2 column 40: datatype sort E declares arity 1 but defines 2 parameters\")\nafter\n"
	parseErrorScripts["datatype_par_beyond_header.smt2"] = true
}
