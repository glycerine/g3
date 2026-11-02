package smt2

// Source: src/muz/bmc/dl_bmc_engine.cpp bmc::query uses default
// transformations and slicing, without Spacer's coalescing/unfolding stage.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2BMCUnfoldOptionDoesNotChangeBoundedSearch(t *testing.T) {
	for _, rounds := range []int{0, 2} {
		t.Run(fmt.Sprint(rounds), func(t *testing.T) {
			c, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := c.Close(); err != nil {
					t.Error(err)
				}
			}()
			script := fmt.Sprintf(`(set-option :fixedpoint.engine bmc)
(set-option :fixedpoint.bmc.linear_unrolling_depth 3)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.subsumption_checker false)
(set-option :fixedpoint.xform.slice false)
(set-option :fixedpoint.xform.unfold_rules %d)
(set-option :fixedpoint.xform.coalesce_rules true)
(set-logic HORN)
(declare-rel P (Int))
(declare-rel G ())
(declare-var x Int)
(rule (P 0))
(rule (=> (and (P x) (< x 3)) (P (+ x 1))))
(rule (=> (P 3) G))
(query G)`, rounds)
			var output bytes.Buffer
			if err := ParseCommands(c, strings.NewReader(script), &output); err != nil {
				t.Fatal(err)
			}
			if got := output.String(); got != "unknown\n" {
				t.Fatalf("got %q, want bounded unknown", got)
			}
		})
	}
}

func TestSMT2BMCSpacerOptionsPreserveCertificates(t *testing.T) {
	for _, engine := range []string{"bmc", "qbmc"} {
		for _, slice := range []string{"false", "true"} {
			t.Run(engine+"/slice="+slice, func(t *testing.T) {
				var baseline string
				for _, options := range []struct {
					unfold   int
					coalesce bool
				}{{0, false}, {2, false}, {2, true}} {
					c, err := cmd_context.NewStandaloneContext()
					if err != nil {
						t.Fatal(err)
					}
					script := fmt.Sprintf(`(set-option :fixedpoint.engine %s)
(set-option :fixedpoint.bmc.linear_unrolling_depth 8)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.subsumption_checker false)
(set-option :fixedpoint.xform.slice %s)
(set-option :fixedpoint.xform.unfold_rules %d)
(set-option :fixedpoint.xform.coalesce_rules %t)
(declare-rel P (Int))
(declare-rel Q (Int))
(declare-rel Goal ())
(declare-var x Int)
(declare-var y Int)
(rule (P x))
(rule (Q y))
(rule (=> (and (P x) (Q y)) Goal))
(query Goal :print-certificate true)`, engine, slice, options.unfold, options.coalesce)
					var output bytes.Buffer
					err = ParseCommands(c, strings.NewReader(script), &output)
					closeErr := c.Close()
					if err != nil {
						t.Fatal(err)
					}
					if closeErr != nil {
						t.Fatal(closeErr)
					}
					got := output.String()
					if !strings.HasPrefix(got, "sat\n") || !strings.Contains(got, "hyper-res") || !strings.Contains(got, "(P ") || strings.Contains(got, "!slice!") {
						t.Fatalf("missing original-rule certificate: %s", got)
					}
					if options.unfold == 0 {
						baseline = got
					} else if got != baseline {
						t.Fatalf("Spacer options changed BMC certificate (unfold=%d coalesce=%t):\n%s\nwant:\n%s", options.unfold, options.coalesce, got, baseline)
					}
				}
			})
		}
	}
}
