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
	// Expected output comes from the installed C++ Z3 reference.
	want := `sat
(let ((a!1 ((_ hyper-res 0 1)
             (asserted (=> G query!0))
             (asserted (=> (P 3) G))
             (=> (P 3) query!0)))
      (a!2 (forall ((A Int) (B Int))
             (=> (and (P B) (not (<= 3 B)) (= A (+ 1 B))) (P A)))))
(let ((a!3 (hyper-res (hyper-res a!1 (asserted a!2) (=> (P 2) query!0))
                      (asserted a!2)
                      (=> (P 1) query!0))))
  ((_ hyper-res 0 1)
    (hyper-res a!3 (asserted a!2) (=> (P 0) query!0))
    (asserted (P 0))
    query!0)))

`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != want {
		t.Fatalf("recursive BMC certificate:\n%s\nwant:\n%s", got, want)
	}
}
