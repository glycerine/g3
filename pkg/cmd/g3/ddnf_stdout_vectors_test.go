package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// ddnfStdoutVectors holds ddnf vectors whose engine output goes straight to
// os.Stdout, as the C++ writes the rejected rule (pre_process_rule) and the
// compiled rules (dump_rules) to std::cout. dump is what the engine writes
// to os.Stdout, out what the runner writes; z3 prints dump followed by out.
var ddnfStdoutVectors = map[string]struct{ dump, out string }{
	// AUDIT.md muz_small_01-F1: process_todo pops the last connective
	// argument first, which fixes the DDNF node numbering.
	"fp_ddnf_or_traversal_order.smt2": {
		dump: "(declare-rel q ())\n(declare-rel p ((_ BitVec 2)))\n(declare-var A (_ BitVec 2))\n(rule (=> (or (= A #b10) (= A #b01)) (p A)))\n(rule (=> (and (p A) (= A #b01)) q))\n(query q)\n",
		out:  "unknown\ndone\n",
	},
	// AUDIT.md muz_small_01-F2: DDNF tests the basic family, so arithmetic
	// > is rejected (the "Could not handle" line goes to stderr).
	"fp_ddnf_arith_gt_rejected.smt2": {
		dump: "<null>:\nq() :- \n p(#0,#1),\n (> (:var 1) 2),\n (= (:var 0) #x1).\n",
		out:  "unknown\ndone\n",
	},
}

func runDDNFStdoutVector(t *testing.T, name string) (dump, out string) {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("test_vectors", name))
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(t.TempDir(), "ddnf-stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	saved := os.Stdout
	os.Stdout = file
	var output bytes.Buffer
	err = run(bytes.NewReader(script), &output)
	os.Stdout = saved
	if err != nil {
		t.Fatal(err, output.String())
	}
	dumped, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(dumped), output.String()
}

func TestDDNFStdoutVectors(t *testing.T) {
	for name, want := range ddnfStdoutVectors {
		t.Run(name, func(t *testing.T) {
			dump, out := runDDNFStdoutVector(t, name)
			if dump != want.dump || out != want.out {
				t.Fatalf("ddnf stdout\n%s\nwant\n%s\nrunner output\n%s\nwant\n%s", dump, want.dump, out, want.out)
			}
		})
	}
}

func TestDDNFStdoutVectorsCPPReference(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to the C++ z3 executable")
	}
	for name, want := range ddnfStdoutVectors {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(reference, filepath.Join("test_vectors", name))
			cppOutput, err := cmd.Output()
			if err != nil {
				t.Fatal("C++ reference", err, string(cppOutput))
			}
			if string(cppOutput) != want.dump+want.out {
				t.Fatalf("C++ output\n%s\nwant\n%s", cppOutput, want.dump+want.out)
			}
		})
	}
}
