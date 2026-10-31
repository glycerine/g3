package duality

import "testing"

func TestDualitySolverOptionsSourceDefaultsAndParsing(t *testing.T) {
	o := NewSolverOptions()
	if o.FullExpand || o.NoConj || !o.FeasibleEdges || !o.UseUnderapprox || o.Report || o.StratifiedInlining || o.RecursionBound != -1 || o.BatchExpand {
		t.Fatal("source defaults", o)
	}
	for _, name := range []string{"full_expand", "no_conj", "feasible_edges", "use_underapprox", "report", "stratified_inlining", "batch_expand", "enable_restarts"} {
		if !o.SetOption(name, "1") || o.SetOption(name, "true") || !o.SetOption(name, "0") {
			t.Fatal("boolean option", name)
		}
	}
	if !o.SetOption("recursion_bound", "  -42tail") || o.RecursionBound != -42 {
		t.Fatal("decimal prefix", o.RecursionBound)
	}
	if !o.SetOption("recursion_bound", "none") || o.RecursionBound != 0 {
		t.Fatal("nonnumeric bound", o.RecursionBound)
	}
	if o.SetOption("conjecture_file", "proof.txt") || o.ConjectureFile != "proof.txt" {
		t.Fatal("conjecture source return", o.ConjectureFile)
	}
	if o.SetOption("unknown", "1") {
		t.Fatal("unknown option")
	}
}
