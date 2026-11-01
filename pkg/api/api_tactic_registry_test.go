package api

import (
	"reflect"
	"testing"
)

// This is the source registration order from the public tactic catalog in
// src/tactic and the logic-specific registrations installed by tactic_cmds.
func TestAPITacticRegistryCountIndexAndNames(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	want := []string{
		"ackermannize_bv", "subpaving", "horn", "horn-simplify", "nlsat", "qfnra-nlsat", "nlqsat", "qe-light",
		"qe-sat", "qe", "qsat", "qe2", "qe_rec", "sat", "sat-preprocess", "ctx-solver-simplify", "smt",
		"unit-subsume-simplify", "aig", "add-bounds", "card2bv", "degree-shift", "diff-neq", "elim01", "eq2bv",
		"factor", "fix-dl-var", "fm", "lia2card", "lia2pb", "nla2bv", "normalize-bounds", "pb2bv",
		"propagate-ineqs", "purify-arith", "recover-01", "bit-blast", "bv1-blast", "bv_bound_chk", "propagate-bv-bounds", "propagate-bv-bounds-new",
		"reduce-bv-size", "bvarray2uf", "dt2bv", "elim-small-bv", "max-bv-sharing", "blast-term-ite", "cofactor-term-ite", "collect-statistics",
		"ctx-simplify", "der", "distribute-forall", "dom-simplify", "elim-term-ite", "elim-uncnstr", "injectivity", "snf",
		"nnf", "occf", "pb-preprocess", "propagate-values", "reduce-args", "simplify", "elim-and", "solve-eqs",
		"split-clause", "symmetry-reduce", "tseitin-cnf", "tseitin-cnf-core", "fpa2bv", "qffp", "qffpbv", "qffplra",
		"nl-purify", "default", "sine-filter", "qfbv-sls", "nra", "qfaufbv", "qfauflia", "qfbv",
		"qfidl", "qflia", "qflra", "qfnia", "qfnra", "qfuf", "qfufbv", "qfufbv_ackr",
		"qfufnra", "ufnia", "uflra", "auflia", "auflira", "aufnira", "lra", "lia",
		"lira", "skip", "fail", "fail-if-undecided", "macro-finder", "quasi-macros", "ufbv-rewriter", "bv",
		"ufbv",
	}

	names, err := f.c.TacticNames()
	astTestMust(t, err)
	count, err := f.c.TacticCount()
	astTestMust(t, err)
	if count != uint32(len(want)) || !reflect.DeepEqual(names, want) {
		t.Fatalf("tactic catalog differs from source: count=%d names=%d", count, len(names))
	}
	for index, expected := range want {
		name, err := f.c.TacticName(uint32(index))
		astTestMust(t, err)
		if name != expected {
			t.Fatalf("tactic name at %d = %q, want %q", index, name, expected)
		}
		if description, err := f.c.TacticDescription(name); err != nil || description == "" {
			t.Fatalf("tactic description for %q = %q, %v", name, description, err)
		}
		created, err := f.c.Tactic(name)
		astTestMust(t, err)
		astTestMust(t, created.Close())
	}
	for _, index := range []uint32{count, ^uint32(0)} {
		if _, err := f.c.TacticName(index); err == nil {
			t.Errorf("out-of-range tactic index %d accepted", index)
		}
	}
	for _, name := range []string{"not-a-retained-tactic", "nlqe"} {
		if yes, err := f.c.HasTactic(name); err != nil || yes {
			t.Errorf("unregistered tactic %q: found=%v err=%v", name, yes, err)
		}
		if _, err := f.c.TacticDescription(name); err == nil {
			t.Errorf("unregistered tactic %q has a description", name)
		}
		if _, err := f.c.Tactic(name); err == nil {
			t.Errorf("unregistered tactic %q was constructed", name)
		}
	}
}
