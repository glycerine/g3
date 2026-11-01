package smt2

import (
	"bytes"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMT2QFIDLStrategicDIMACSFollowsCurrentOutput(t *testing.T) {
	testArithmeticStrategicDIMACS(t, "QF_IDL")
}

func TestSMT2QFLIAStrategicDIMACSFollowsCurrentOutput(t *testing.T) {
	testArithmeticStrategicDIMACS(t, "QF_LIA")
}

func TestSMT2ParseAssertionsIgnoresCheckCommands(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	script := `(set-logic QF_UF)
(declare-const p Bool)
(assert p)
(check-sat)
(check-sat-using smt)`
	if err = ParseAssertions(c, strings.NewReader(script)); err != nil {
		t.Fatal(err)
	}
	assertions, err := c.Assertions()
	if err != nil || len(assertions) != 1 {
		t.Fatal("final assertions", len(assertions), err)
	}
	reason, err := c.ReasonUnknown()
	if err != nil || reason != "state of the most recent check-sat command is not known" {
		t.Fatal("check command executed", reason, err)
	}
}

func TestSMT2EvalCommandMatchesCPP(t *testing.T) {
	script := `(set-logic QF_UF)(declare-const p Bool)(check-sat)
(eval p)(eval p :completion true)(eval p)
(eval p :cache false :array_equalities false :max_memory 4294967295 :max_steps 4294967295 :timeout 4294967295 :rlimit 4294967295)
(eval (not p) :max_steps 0)`
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	var output bytes.Buffer
	if err = ParseCommands(c, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if want := "sat\np\nfalse\nfalse\nfalse\n(error \"evaluator failed: max. steps exceeded\")\n"; output.String() != want {
		t.Fatal("eval output", output.String(), want)
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		command := exec.Command(reference, "-in")
		command.Stdin = strings.NewReader(script)
		cpp, err := command.Output()
		if err != nil {
			t.Fatal(err, string(cpp))
		}
		if output.String() != string(cpp) {
			t.Fatal("C++ eval differential", output.String(), string(cpp))
		}
	}
}

func testArithmeticStrategicDIMACS(t *testing.T, logic string) {
	t.Helper()
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	setup := `(set-logic ` + logic + `)
(set-option :sat.dimacs.display true)
(declare-const a Int)(declare-const b Int)(declare-const c Int)
(assert (and (>= a 0)(<= a 1)(>= b 0)(<= b 1)(>= c 0)(<= c 1)))
(assert (or (= a 1)(= b 1)))
(assert (or (= a 0)(= b 1)))
(assert (or (= b 0)(= c 1)))
(assert (or (= a 0)(= c 0)))
`
	var initial, first, second bytes.Buffer
	if err := ParseCommands(c, strings.NewReader(setup), &initial); err != nil {
		t.Fatal(err)
	}
	// The strategy is already constructed with the initial parser writer.
	// Each subsequent parse must supply the current transport to its SAT child.
	firstLength := 0
	for _, output := range []*bytes.Buffer{&first, &second} {
		if err := ParseCommands(c, strings.NewReader("(check-sat)"), output); err != nil {
			t.Fatal(err, output.String())
		}
		if !strings.Contains(output.String(), "p cnf ") || !strings.HasSuffix(output.String(), "unknown\n") {
			t.Fatal("missing DIMACS/refusal in the current parser output", output.String())
		}
		if output == &first {
			firstLength = first.Len()
		}
	}
	if initial.Len() != 0 || first.Len() != firstLength {
		t.Fatal("strategy retained an earlier output transport", initial.String(), first.String(), second.String())
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		cmd := exec.Command(reference, "-in")
		cmd.Stdin = strings.NewReader(setup + "(check-sat)\n(check-sat)\n")
		var output, diagnostics bytes.Buffer
		cmd.Stdout, cmd.Stderr = &output, &diagnostics
		if err := cmd.Run(); err != nil {
			t.Fatal(err, output.String(), diagnostics.String())
		}
		// Source goal2sat prints its inverse atom map in hash-table order.
		// Compare every mapping within each dump, preserving clause order and
		// the separation between the two independent solver applications.
		normalize := func(text string) string {
			lines := strings.Split(text, "\n")
			for i := 0; i < len(lines); {
				if !strings.HasPrefix(lines[i], "c ") {
					i++
					continue
				}
				end := i + 1
				for end < len(lines) && strings.HasPrefix(lines[end], "c ") {
					end++
				}
				sort.Strings(lines[i:end])
				i = end
			}
			return strings.Join(strings.Fields(strings.Join(lines, "\n")), " ")
		}
		if normalize(output.String()) != normalize(first.String()+second.String()) {
			t.Fatal("DIMACS/refusal differs from C++", output.String(), first.String()+second.String())
		}
	}
}

func TestSMT2CommandsRejectMalformedInputBeforeEffects(t *testing.T) {
	for _, command := range []string{"(assert false extra)", "(declare-fun p () Bool extra)", "(declare-const p (Array))", "(declare-const p (Array Bool))", "(assert (forall ((x Bool))", "(get-info)", "(get-info reason-unknown)", "(get-info :reason-unknown extra)", "(get-info :reason-unknown"} {
		t.Run(command, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := context.Close(); err != nil {
					t.Fatal(err)
				}
			}()
			var out bytes.Buffer
			if err := ParseCommands(context, strings.NewReader(command), &out); err == nil {
				t.Fatal("malformed command was accepted")
			}
			if strings.HasPrefix(command, "(get-info") && out.Len() != 0 {
				t.Fatal("malformed query emitted output", out.String())
			}
			if strings.HasPrefix(command, "(assert") {
				status, err := context.CheckSat(nil)
				if err != nil || status != lbool.True {
					t.Fatal("malformed assertion changed solver state", status, err)
				}
			}
			if strings.HasPrefix(command, "(declare") {
				if _, err := context.MakeApp("p", nil, nil, ast.ASTHandle{}); err == nil {
					t.Fatal("malformed declaration was installed")
				}
			}
		})
	}
}

func TestSMT2ExitStopsBeforeTrailingInputAndPrintsSuccess(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	var out bytes.Buffer
	if err := ParseCommands(context, strings.NewReader("(set-option :print-success true)(exit)["), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "success\nsuccess\n" {
		t.Fatal(out.String())
	}
}

func TestSMT2GetValuePreservesParsedTermTextAcrossQueries(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	input := `(set-logic QF_LIA)
(set-option :produce-models true)
(define-fun |a b| () Int 3)
(define-fun id ((n Int)) Int n)
(check-sat)
(get-value (|a b| (id   (+ 1 2)) (let ((q |a b|)) (+ q 0))))
(get-value ((+ 4 5)))`
	var out bytes.Buffer
	if err := ParseCommands(context, strings.NewReader(input), &out); err != nil {
		t.Fatal(err, out.String())
	}
	want := "sat\n((|a b| 3)\n ((id   (+ 1 2)) 3)\n ((let ((q |a b|)) (+ q 0)) 3))\n(((+ 4 5) 9))\n"
	if out.String() != want {
		t.Fatal("get-value must echo the original terms", out.String(), want)
	}
}

func TestSMT2GetValueRejectsEmptyAndQuantifiedTermsBeforeOutput(t *testing.T) {
	for _, command := range []string{"(get-value ())", "(get-value ((forall ((x Bool)) x)))", "(get-value ((and true (exists ((x Bool)) x))))"} {
		t.Run(command, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Fatal(err)
				}
			})
			var out bytes.Buffer
			err = ParseCommands(context, strings.NewReader("(set-option :produce-models true)(check-sat)"+command), &out)
			if err == nil || !strings.Contains(err.Error(), "invalid get-value") || out.String() != "sat\n" {
				t.Fatal("invalid query was accepted or partially printed", err, out.String())
			}
		})
	}
}

func TestSMT2UnsignedOptionsAndLiveRelevancyUpdates(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	input := `(set-option :smt.relevancy 0)
(set-option :rlimit 4294967295)
(set-option :produce-models true)
(declare-const p Bool)
(assert p)
(check-sat)
(get-value (p))
(set-option :smt.relevancy 2)
(check-sat)
(get-value (p))`
	var out bytes.Buffer
	if err := ParseCommands(context, strings.NewReader(input), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if want := "sat\n((p true))\nsat\n((p true))\n"; out.String() != want {
		t.Fatal("unsigned options or relevancy update", out.String(), want)
	}
}

func TestSMT2UnsignedOptionsRejectRangeAndTrailingArguments(t *testing.T) {
	for _, value := range []string{"4294967296", "0 extra", "0.5", "-1"} {
		t.Run(value, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Fatal(err)
				}
			})
			var out bytes.Buffer
			if err := ParseCommands(context, strings.NewReader("(set-option :smt.relevancy "+value+")"), &out); err == nil || out.Len() != 0 {
				t.Fatal("invalid unsigned option accepted or printed output", err, out.String())
			}
		})
	}
}

func TestSMT2RelevancyUpdatesRestoreCachedBooleanAndTermITEWatches(t *testing.T) {
	for _, formula := range []string{"(or a b)", "(not (and a b))", "(ite c a b)", "(= (ite c 1 2) x)"} {
		t.Run(formula, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Fatal(err)
				}
			})
			query := "(check-sat)(get-value (" + formula + "))"
			input := "(set-option :smt.relevancy 0)(set-option :produce-models true)" +
				"(declare-const a Bool)(declare-const b Bool)(declare-const c Bool)(declare-const x Int)" +
				"(assert " + formula + ")" + query +
				"(push 1)(set-option :smt.relevancy 2)" + query +
				"(push 1)(assert (not c))" + query +
				"(pop 1)" + query + "(pop 1)" + query
			var out bytes.Buffer
			if err := ParseCommands(context, strings.NewReader(input), &out); err != nil {
				t.Fatal(err, out.String())
			}
			want := strings.Repeat("sat\n(("+formula+" true))\n", 5)
			if out.String() != want {
				t.Fatal("cached relevance watches lost an asserted gate or term ITE", out.String(), want)
			}
		})
	}
}

func TestSMT2FunctionDefinitionFailuresAndRecovery(t *testing.T) {
	for _, script := range []string{
		"(define-fun f ((x Int)) Real x)",
		"(define-const f Real 1)",
		"(define-fun f ((x Bool)) Bool x extra)",
		"(define-fun f ((x Bool)) Bool x)(define-fun f ((y Bool)) Bool (not y))",
		"(declare-fun f (Bool) Bool)(define-fun f ((x Bool)) Bool x)",
		"(define-fun f ((x Bool)) Bool x)(declare-fun f (Bool) Bool)",
		"(define-fun and ((x Bool)) Bool x)",
	} {
		t.Run(script, func(t *testing.T) {
			c, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := c.Close(); err != nil {
					t.Fatal(err)
				}
			}()
			var out bytes.Buffer
			if err := ParseCommands(c, strings.NewReader(script), &out); err == nil {
				t.Fatal("invalid definition accepted")
			}
			// A failed definition must leave the command context usable, including
			// releasing parser locals and restoring bound-variable scope.
			if err := ParseCommands(c, strings.NewReader("(define-fun g ((x Bool)) Bool x)(assert (g true))(check-sat)"), &out); err != nil {
				t.Fatal(err)
			}
			if out.String() != "sat\n" {
				t.Fatal(out.String())
			}
		})
	}
}
