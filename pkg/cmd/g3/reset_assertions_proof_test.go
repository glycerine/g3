package main

import (
	"bytes"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestSMTLibRunnerResetAssertionsPreservesProofResultsCPPReference(t *testing.T) {
	script, err := os.ReadFile("test_vectors/reset_assertions_proofs.smt2")
	if err != nil {
		t.Fatal(err)
	}
	b := arena.NewBudget(1 << 30)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	if err != nil {
		t.Fatal(err)
	}
	check := func(output []byte) {
		t.Helper()
		scanner, err := smt2.NewScanner(bytes.NewReader(output), false, numerals.Manager)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := scanner.Close(); err != nil {
				t.Fatal(err)
			}
		}()
		events := []string{}
		for {
			token, err := scanner.Scan()
			if err != nil {
				t.Fatal(err)
			}
			if token == smt2.EOFToken {
				break
			}
			if token == smt2.SymbolToken {
				events = append(events, scanner.ID())
				continue
			}
			if token != smt2.LeftParenToken {
				t.Fatal("unexpected proof-output token", token)
			}
			depth := 1
			asserted, falsehood := false, false
			for depth != 0 {
				token, err = scanner.Scan()
				if err != nil {
					t.Fatal(err)
				}
				switch token {
				case smt2.EOFToken:
					t.Fatal("unterminated proof")
				case smt2.LeftParenToken:
					depth++
				case smt2.RightParenToken:
					depth--
				case smt2.SymbolToken:
					asserted = asserted || scanner.ID() == "asserted"
					falsehood = falsehood || scanner.ID() == "false"
				}
			}
			if !asserted || !falsehood {
				t.Fatal("incomplete reset-assertions proof", string(output))
			}
			events = append(events, "proof")
		}
		want := []string{"unsat", "proof", "proof", "sat", "unsat", "proof", "sat"}
		if !reflect.DeepEqual(events, want) {
			t.Fatal("lost retained or replacement proof session", events, string(output))
		}
	}
	var out bytes.Buffer
	if err = run(bytes.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	check(out.Bytes())
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		cmd := exec.Command(reference, "-in")
		cmd.Stdin = bytes.NewReader(script)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err = cmd.Run(); err != nil {
			t.Fatal(err, stdout.String(), stderr.String())
		}
		check(stdout.Bytes())
	}
}
