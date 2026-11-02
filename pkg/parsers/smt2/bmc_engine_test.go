package smt2

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2BMCEngineSearch(t *testing.T) {
	for _, tc := range []struct{ name, options, rules, want string }{
		{"linear", "", "(rule A)(rule (=> A G))", "sat\n"},
		{"bounded", "(set-option :fixedpoint.bmc.linear_unrolling_depth 0)", "(rule A)(rule (=> A G))", "unknown\n"},
		{"bounded-after-inlining", "(set-option :fixedpoint.bmc.linear_unrolling_depth 1)", "(rule A)(rule (=> A G))", "sat\n"},
		{"bounded-without-inlining", "(set-option :fixedpoint.bmc.linear_unrolling_depth 1)(set-option :fixedpoint.xform.inline_eager false)(set-option :fixedpoint.xform.inline_linear false)", "(rule A)(rule (=> A G))", "sat\n"},
		{"nonlinear", "", "(rule A)(rule B)(rule (=> (and A B) G))", "sat\n"},
		{"no-rules", "", "", "unsat\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			}()
			script := fmt.Sprintf("(set-option :fixedpoint.engine bmc)%s(set-logic HORN)(declare-rel A ())(declare-rel B ())(declare-rel G ())%s(query G)", tc.options, tc.rules)
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err)
			}
			if got := output.String(); got != tc.want {
				t.Fatalf("output = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSMT2BMCCertificate(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(set-option :fixedpoint.engine bmc)
(declare-rel A ())
(declare-rel G ())
(declare-rel U ())
(rule A)
(rule (=> A G))
(query G :print-certificate true)
(query U :print-certificate true)`
	var output bytes.Buffer
	if err := ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "sat\n(asserted query!0)\n\nunsat\nnull\n\n" {
		t.Fatalf("BMC certificates = %q", got)
	}
}

func TestSMT2BMCRecursiveCertificate(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(set-option :fixedpoint.engine bmc)
(set-option :fixedpoint.bmc.linear_unrolling_depth 8)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-logic HORN)
(declare-rel P (Int))
(declare-rel G ())
(declare-var x Int)
(rule (P 0))
(rule (=> (and (P x) (< x 3)) (P (+ x 1))))
(rule (=> (P 3) G))
(query G :print-certificate true)`
	// Keep an exact certificate oracle with valid body/head pivots and explicit
	// arithmetic rewrite proofs; independent recursive proof checks live in
	// fp.TestBMCNumericProofIndependentValidation.
	want := `sat
(let ((a!1 ((_ hyper-res 1 0)
             (asserted (=> G query!0))
             (asserted (=> (P 3) G))
             (=> (P 3) query!0)))
      (a!2 (forall ((A Int) (B Int))
             (=> (and (P B) (not (<= 3 B)) (= A (+ 1 B))) (P A))))
      (a!3 (=> (and (P 2) (not (<= 3 2)) (= 3 (+ 1 2))) query!0))
      (a!5 (=> (and (P 1) (not (<= 3 1)) (= 2 (+ 1 1))) query!0))
      (a!8 (=> (and (P 0) (not (<= 3 0)) (= 1 (+ 1 0))) query!0)))
(let ((a!4 (rewrite (= a!3 (=> (P 2) query!0))))
      (a!7 (rewrite (= a!5 (=> (P 1) query!0))))
      (a!10 (rewrite (= a!8 (=> (P 0) query!0)))))
(let ((a!6 (hyper-res (mp (hyper-res a!1 (asserted a!2) a!3)
                          a!4
                          (=> (P 2) query!0))
                      (asserted a!2)
                      a!5)))
(let ((a!9 (hyper-res (mp a!6 a!7 (=> (P 1) query!0)) (asserted a!2) a!8)))
  ((_ hyper-res 1 0) (mp a!9 a!10 (=> (P 0) query!0)) (asserted (P 0)) query!0)))))

`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != want {
		t.Fatalf("recursive BMC certificate:\n%s\nwant:\n%s", got, want)
	}
}

func TestSMT2BMCNonlinearProofTrace(t *testing.T) {
	for _, engine := range []string{"bmc", "qbmc"} {
		t.Run(engine, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			}()
			script := `(set-option :fixedpoint.engine ` + engine + `)
(set-option :fixedpoint.generate_proof_trace true)
(set-option :fixedpoint.xform.slice false)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.subsumption_checker false)
(declare-rel A ())
(declare-rel B ())
(declare-rel Goal ())
(rule A)
(rule B)
(rule (=> (and A B) Goal))
(query Goal :print-certificate true)
`
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err, output.String())
			}
			text := output.String()
			if !strings.HasPrefix(text, "sat\n") || !strings.Contains(text, "hyper-res") || !strings.Contains(text, "asserted") {
				t.Fatal("nonlinear certificate", text)
			}
		})
	}
}

func TestSMT2BMCNonlinearCertificateWithoutProofOption(t *testing.T) {
	for _, engine := range []string{"bmc", "qbmc"} {
		for _, slice := range []string{"true", "false"} {
			for _, outputKind := range []string{"print-certificate", "print-answer"} {
				t.Run(engine+"/slice="+slice+"/"+outputKind, func(t *testing.T) {
					context, err := cmd_context.NewStandaloneContext()
					if err != nil {
						t.Fatal(err)
					}
					defer func() {
						if err := context.Close(); err != nil {
							t.Error(err)
						}
					}()
					script := fmt.Sprintf(`(set-option :fixedpoint.engine %s)
(set-option :fixedpoint.xform.slice %s)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.subsumption_checker false)
(declare-rel P (Int))
(declare-rel Q (Int))
(declare-rel Goal ())
(declare-var x Int)
(declare-var y Int)
(rule (P x))
(rule (Q y))
(rule (=> (and (P x) (Q y)) Goal))
(query Goal :%s true)
`, engine, slice, outputKind)
					var output bytes.Buffer
					if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
						t.Fatal(err, output.String())
					}
					text := output.String()
					if !strings.HasPrefix(text, "sat\n") || !strings.Contains(text, "hyper-res") || !strings.Contains(text, "(P ") || !strings.Contains(text, "(Q ") || strings.Contains(text, "!slice!") {
						t.Fatal("nonlinear certificate without global proof option", text)
					}
				})
			}
		}
	}
}
