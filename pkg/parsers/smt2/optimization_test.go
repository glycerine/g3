package smt2

// Source: src/opt/opt_cmds.cpp maximize/minimize/get-objectives and
// src/opt/opt_context.cpp scoped arithmetic objectives.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
)

func TestSMT2BoundedIntegerOptimizationEndToEnd(t *testing.T) {
	for _, tc := range []struct {
		command, value string
	}{
		{"maximize", "3"},
		{"minimize", "0"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			})
			script := `(set-option :produce-models true)
(set-logic QF_LIA)
(declare-const x Int)
(assert (and (>= x 0) (<= x 3)))
(` + tc.command + ` x)
(check-sat)
(get-objectives)
(get-value (x))`
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err, output.String())
			}
			want := "sat (objectives (x " + tc.value + ") ) ((x " + tc.value + "))"
			if got := strings.Join(strings.Fields(output.String()), " "); got != want {
				t.Fatal("optimization output", got, "want", want)
			}
		})
	}
}

func TestSMT2GeometricIntegerOptimizationCrossesBooleanBranches(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    string
	}{{"maximize", "1000"}, {"minimize", "0"}} {
		t.Run(tc.command, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			})
			script := `(set-logic QF_LIA)
(declare-const choose-low Bool)(declare-const x Int)
(assert (or (and choose-low (= x 0)) (and (not choose-low) (= x 1000))))
	` + "(" + tc.command + ` x)(check-sat)(get-objectives)(get-value (x))`
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err, output.String())
			}
			want := "sat (objectives (x " + tc.want + ") ) ((x " + tc.want + "))"
			if got := strings.Join(strings.Fields(output.String()), " "); got != want {
				t.Fatal("geometric integer optimization", got, "want", want)
			}
		})
	}
}

func TestSMT2OptimizationObjectivesFollowScopes(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	var output bytes.Buffer
	script := `(set-logic QF_LIA)(declare-const x Int)(assert (= x 1))
(push)(maximize x)(pop)(check-sat)(get-objectives)`
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(strings.Fields(output.String()), " "); got != "sat (objectives )" {
		t.Fatal("popped objective survived", got)
	}
}

func TestSMT2WeightedSoftOptimizationEndToEnd(t *testing.T) {
	tests := []struct {
		name, script, want string
	}{
		{
			"default-group",
			`(set-option :produce-models true)
(set-logic QF_FD)
(declare-const a Bool)(declare-const b Bool)
(assert (not (and a b)))
(assert-soft a)(assert-soft b :weight 2)
(check-sat)(get-objectives)(get-value (a b))`,
			"sat (objectives ( 1) ) ((a false) (b true))",
		},
		{
			"rational-groups",
			`(set-option :produce-models true)
(set-logic QF_FD)
(declare-const a Bool)(declare-const b Bool)
(assert (not (and a b)))
(assert-soft a :weight 1.5 :id x)
(assert-soft b :weight 2.25 :id y)
(check-sat)(get-objectives)(get-value (a b))`,
			"sat (objectives (x 0) (y (/ 9.0 4.0)) ) ((a true) (b false))",
		},
		{
			"negative-weight",
			`(set-option :produce-models true)
(set-logic QF_FD)(declare-const a Bool)
(assert-soft a :weight -2)
(check-sat)(get-objectives)(get-value (a))`,
			"sat (objectives ( (- 2)) ) ((a false))",
		},
		{
			"existing-group-scope",
			`(set-option :produce-models true)
(set-logic QF_FD)(declare-const a Bool)
(assert-soft a :id x)
(push)(assert-soft (not a) :weight 3 :id x)(pop)
(check-sat)(get-objectives)(get-value (a))`,
			"sat (objectives (x 0) ) ((a true))",
		},
		{
			"mixed-lexicographic-order",
			`(set-option :produce-models true)
(set-logic QF_LIA)
(declare-const a Bool)(declare-const x Int)
(assert (and (>= x 0) (<= x 1)))
(assert-soft a :id s)(maximize x)
(assert-soft (not a) :weight 2 :id t)
(check-sat)(get-objectives)(get-value (a x))`,
			"sat (objectives (s 0) (x 1) (t 2) ) ((a true) (x 1))",
		},
		{
			"retained-command-parameters",
			`(set-logic QF_FD)
(declare-const a Bool)(declare-const b Bool)
(assert (not (and a b)))
(assert-soft a :weight 3 :id first)
(assert-soft b)
(check-sat)(get-objectives)`,
			"sat (objectives (first 3) )",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			})
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(tc.script), &output); err != nil {
				t.Fatal(err, output.String())
			}
			if got := strings.Join(strings.Fields(output.String()), " "); got != tc.want {
				t.Fatal("soft optimization output", got, "want", tc.want)
			}
		})
	}
}

func TestSMT2BitVectorOptimizationEndToEnd(t *testing.T) {
	tests := []struct {
		name, script, want string
	}{
		{
			"maximize",
			`(set-option :produce-models true)(set-logic QF_BV)
(declare-const x (_ BitVec 4))(assert (bvule x #x9))
(maximize x)(check-sat)(get-objectives)(get-value (x))`,
			"sat (objectives (x 9) ) ((x #x9))",
		},
		{
			"minimize",
			`(set-option :produce-models true)(set-logic QF_BV)
(declare-const x (_ BitVec 4))(assert (bvuge x #x3))
(minimize x)(check-sat)(get-objectives)(get-value (x))`,
			"sat (objectives (x 3) ) ((x #x3))",
		},
		{
			"lexicographic",
			`(set-option :produce-models true)(set-logic QF_BV)
(declare-const x (_ BitVec 4))(declare-const y (_ BitVec 4))
(assert (= (bvadd x y) #x7))(maximize x)(minimize y)
(check-sat)(get-objectives)(get-value (x y))`,
			"sat (objectives (x 15) (y 8) ) ((x #xf) (y #x8))",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			})
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(tc.script), &output); err != nil {
				t.Fatal(err, output.String())
			}
			if got := strings.Join(strings.Fields(output.String()), " "); got != tc.want {
				t.Fatal("bit-vector optimization output", got, "want", tc.want)
			}
		})
	}
}

func TestSMT2StrictLexicographicIntervalEndToEnd(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	const script = `(set-option :produce-models true)
(set-logic QF_LRA)
(declare-const x Real)(declare-const y Real)
(assert (> x 0.0))(assert (< x 5.0))(assert (= y x))
(maximize x)(minimize y)
(check-sat)(get-objectives)(get-value (x y))`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	want := "sat (objectives (x (interval (/ 9.0 2.0) (+ 5.0 (* (- 1.0) epsilon)))) (y (/ 9.0 2.0)) ) ((x (/ 9.0 2.0)) (y (/ 9.0 2.0)))"
	if got := strings.Join(strings.Fields(output.String()), " "); got != want {
		t.Fatal("strict lexicographic interval output", got, "want", want)
	}
}

func TestSMT2UnboundedLexicographicIntervalEndToEnd(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	const script = `(set-option :produce-models true)(set-logic QF_LRA)
(declare-const x Real)(declare-const y Real)
(maximize x)(minimize y)
(check-sat)(get-objectives)(get-value (x y))`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	want := "sat (objectives (x oo) (y (interval (* (- 1) oo) oo)) ) ((x 0.0) (y 0.0))"
	if got := strings.Join(strings.Fields(output.String()), " "); got != want {
		t.Fatal("unbounded lexicographic interval output", got, "want", want)
	}
}

func TestSMT2BoxOptimizationEndToEnd(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	const script = `(set-option :produce-models true)(set-option :opt.priority box)
(set-logic QF_LIA)(declare-const x Int)(declare-const y Int)
(assert (and (>= x 0) (>= y 0) (= (+ x y) 5)))
(maximize x)(maximize y)
(check-sat)(get-objectives)(get-value (x y))(eval x :model_index 1)(eval y :model-index 1)
(check-sat)(get-objectives)(get-value (x y))(check-sat)`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	want := "sat (objectives (x 5) (y 5) ) ((x 5) (y 0)) 0 5 sat (objectives (x 5) (y 5) ) ((x 0) (y 5)) unsat"
	if got := strings.Join(strings.Fields(output.String()), " "); got != want {
		t.Fatal("box optimization output", got, "want", want)
	}
}

func TestSMT2ParetoOptimizationEndToEnd(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	var script strings.Builder
	script.WriteString(`(set-option :produce-models true)(set-option :opt.priority pareto)
(set-logic QF_LIA)(declare-const x Int)(declare-const y Int)
(assert (and (>= x 0) (>= y 0) (= (+ x y) 5)))
(maximize x)(maximize y)`)
	for i := 0; i < 6; i++ {
		script.WriteString(`(check-sat)(get-value (x y))`)
	}
	script.WriteString(`(check-sat)`)
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script.String()), &output); err != nil {
		t.Fatal(err, output.String())
	}
	got := strings.Join(strings.Fields(output.String()), " ")
	satCount := 0
	for _, field := range strings.Fields(output.String()) {
		if field == "sat" {
			satCount++
		}
	}
	if satCount != 6 || !strings.HasSuffix(got, "unsat") {
		t.Fatal("Pareto status sequence", got)
	}
	for x := 0; x <= 5; x++ {
		point := "((x " + fmt.Sprint(x) + ") (y " + fmt.Sprint(5-x) + "))"
		if strings.Count(got, point) != 1 {
			t.Fatal("Pareto frontier point", point, got)
		}
	}
}

func TestSMT2BitVectorParetoOptimizationEndToEnd(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	var script strings.Builder
	script.WriteString(`(set-option :produce-models true)(set-option :opt.priority pareto)
(set-logic QF_BV)(declare-const x (_ BitVec 2))(declare-const y (_ BitVec 2))
(assert (= (bvadd x y) #b11))(maximize x)(maximize y)`)
	for i := 0; i < 4; i++ {
		script.WriteString(`(check-sat)(get-objectives)(get-value (x y))`)
	}
	script.WriteString(`(check-sat)`)
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script.String()), &output); err != nil {
		t.Fatal(err, output.String())
	}
	got := strings.Join(strings.Fields(output.String()), " ")
	satCount := 0
	for _, field := range strings.Fields(output.String()) {
		if field == "sat" {
			satCount++
		}
	}
	if satCount != 4 || !strings.HasSuffix(got, "unsat") {
		t.Fatal("bit-vector Pareto status sequence", got)
	}
	for x := 0; x < 4; x++ {
		point := fmt.Sprintf("((x #b%02b) (y #b%02b))", x, 3-x)
		if strings.Count(got, point) != 1 {
			t.Fatal("bit-vector Pareto frontier point", point, got)
		}
		objective := fmt.Sprintf("(objectives (x %d) (y %d) )", x, 3-x)
		if strings.Count(got, objective) != 1 {
			t.Fatal("bit-vector Pareto objective", objective, got)
		}
	}
}

func TestSMT2SoftParetoOptimizationEndToEnd(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	script := `(set-option :produce-models true)(set-option :opt.priority pareto)
(set-logic QF_FD)(declare-const a Bool)(declare-const b Bool)
(assert (not (and a b)))(assert-soft a :id x)(assert-soft b :weight 1 :id y)
(check-sat)(get-objectives)(get-value (a b))
(check-sat)(get-objectives)(get-value (a b))(check-sat)`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	got := strings.Join(strings.Fields(output.String()), " ")
	for _, want := range []string{
		"(objectives (x 0) (y 1) ) ((a true) (b false))",
		"(objectives (x 1) (y 0) ) ((a false) (b true))",
	} {
		if strings.Count(got, want) != 1 {
			t.Fatal("soft Pareto frontier point", want, got)
		}
	}
	if !strings.HasSuffix(got, "unsat") {
		t.Fatal("soft Pareto terminal status", got)
	}
}

func TestSMT2NegativeSoftParetoOptimizationEndToEnd(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	script := `(set-option :produce-models true)(set-option :opt.priority pareto)
(set-logic QF_FD)(declare-const a Bool)
(assert-soft a :weight -2 :id x)
(check-sat)(get-objectives)(get-value (a))(check-sat)`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	got := strings.Join(strings.Fields(output.String()), " ")
	want := "sat (objectives (x (- 2)) ) ((a false)) unsat"
	if got != want {
		t.Fatal("negative soft Pareto normalization", got, "want", want)
	}
}

func TestSMT2AssertSoftNormalizesOptionsAndValidatesFormulaFirst(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	}()
	var output bytes.Buffer
	script := `(set-logic QF_FD)
(declare-const x Bool)
(assert (not x))
(assert-soft x :WEIGHT 2 :ID group)
(check-sat)
(get-objectives)`
	if err = ParseCommands(c, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if got := strings.Join(strings.Fields(output.String()), " "); got != "sat (objectives (group 2) )" {
		t.Fatal(got)
	}
	other, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := other.Close(); err != nil {
			t.Error(err)
		}
	}()
	err = ParseCommands(other, strings.NewReader(`(assert-soft 3 :junk 2)`), &output)
	if err == nil || !strings.Contains(err.Error(), "expected Boolean type") {
		t.Fatal("non-Boolean formula should be rejected first", err)
	}
}

func TestSMT2ObjectivePublishesBeforeExtraArgumentError(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	}()
	var output bytes.Buffer
	if err = ParseCommands(c, strings.NewReader(`(set-option :print-success true)(declare-const x Int)`), &output); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err = ParseCommands(c, strings.NewReader(`(maximize x extra)`), &output); err == nil {
		t.Fatal("extra argument was accepted")
	}
	if output.String() != "success\n" {
		t.Fatalf("source success prefix: %q", output.String())
	}
	output.Reset()
	if err = ParseCommands(c, strings.NewReader(`(check-sat)(get-objectives)`), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if got := strings.Join(strings.Fields(output.String()), " "); got != "sat (objectives (x oo) )" {
		t.Fatal("objective did not survive command error", got)
	}
}
