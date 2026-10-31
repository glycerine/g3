package arith

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func fmTraceRecord(line int, body string) string {
	return fmt.Sprintf("-------- [fm_mc] operator() src/tactic/arith/fm_tactic.cpp:%d ---------\n%s------------------------------------------------\n", line, body)
}
func TestFMModelTraceSourceRecordsAndClauseCheckpoints(t *testing.T) {
	c, m := fmModelFixture(t)
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("fm_mc")
	c.services.Trace = tags
	x := fixDLConst(t, m, false, "x")
	decl, err := m.GetDecl(x)
	equationMust(t, err)
	equationMust(t, c.Insert(decl, []ast.ASTHandle{fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "2")), fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4"))}))
	md, err := model.NewModel(m)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, md.Close()) })
	before := m.ResourceLimit().Count()
	core := fmApply(t, c, md)
	if got := fmValue(t, m, core, x); got != "3" {
		t.Fatal(got)
	}
	if delta := m.ResourceLimit().Count() - before; delta != 2 {
		t.Fatal("one checkpoint per saved clause", delta)
	}
	want := fmTraceRecord(184, "(fm-model-converter\n(x\n  (>= x 2.0)\n  (<= x 4.0)))\n") +
		fmTraceRecord(197, "processing x\n") +
		fmTraceRecord(207, "lower bound: 2 for:\n(>= x 2.0)\n") +
		fmTraceRecord(213, "upper bound: 4 for:\n(<= x 4.0)\n") +
		fmTraceRecord(240, "x --> 3.0\n") +
		fmTraceRecord(243, "x -> 3.0\n")
	if tags.ShouldTrace("fm_mc") {
		if out.String() != want {
			t.Fatalf("source model trace\ngot %q\nwant %q", out.String(), want)
		}
	} else if out.Len() != 0 {
		t.Fatal("erased trace produced output", out.String())
	}
}

func TestFMModelTraceNoBoundAndRefusedStream(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		t.Run(fmt.Sprint(refuse), func(t *testing.T) {
			c, m := fmModelFixture(t)
			var out bytes.Buffer
			var writer io.Writer = &out
			if refuse {
				writer = fixDLRefusedWriter{}
			}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			tags.Enable("fm_mc")
			c.services.Trace = tags
			x := fixDLConst(t, m, false, "x")
			boolean, err := m.MkBoolSort()
			equationMust(t, err)
			name, err := m.Symbols.Intern("p")
			equationMust(t, err)
			p, err := m.MkConst(name, boolean)
			equationMust(t, err)
			fixDLPin(t, m, p)
			bound := fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4"))
			clause, err := m.MkOrArgs([]ast.ASTHandle{p, bound})
			equationMust(t, err)
			fixDLPin(t, m, clause)
			decl, err := m.GetDecl(x)
			equationMust(t, err)
			equationMust(t, c.Insert(decl, []ast.ASTHandle{clause}))
			md, err := model.NewModel(m)
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, md.Close()) })
			pd, err := m.GetDecl(p)
			equationMust(t, err)
			yes, err := m.MkTrue()
			equationMust(t, err)
			equationMust(t, md.Core().RegisterConstant(pd, yes))
			core := fmApply(t, c, md)
			if got := fmValue(t, m, core, x); got != "0" {
				t.Fatal("satisfied literal must disable bound", got)
			}
			if !refuse && tags.ShouldTrace("fm_mc") && !strings.Contains(out.String(), fmTraceRecord(204, "no bound for:\n(or p (<= x 4.0))\n")) {
				t.Fatal("missing NONE record", out.String())
			}
		})
	}
}
func TestFMModelTraceOperandAllocationFailure(t *testing.T) {
	c, m := fmModelFixture(t)
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("fm_mc")
	c.services.Trace = tags
	x := fixDLConst(t, m, false, "x")
	decl, err := m.GetDecl(x)
	equationMust(t, err)
	equationMust(t, c.Insert(decl, []ast.ASTHandle{fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4"))}))
	md, err := model.NewModel(m)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, md.Close()) })
	var ref tactic.ModelRef
	equationMust(t, ref.Set(md.Core()))
	t.Cleanup(func() { equationMust(t, ref.Close()) })
	m.Budget().FailAfter(0)
	err = c.ApplyModel(&ref, 0)
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("operand allocation error suppressed", err)
	}
	if tags.ShouldTrace("fm_mc") && !strings.Contains(out.String(), "(fm-model-converter\n(x\n  ") {
		t.Fatal("failed before reached trace operand", out.String())
	}
}

func TestFMModelClauseCancellationAndReversePrefix(t *testing.T) {
	c, m := fmModelFixture(t)
	x := fixDLConst(t, m, false, "x")
	y := fixDLConst(t, m, false, "y")
	xd, err := m.GetDecl(x)
	equationMust(t, err)
	yd, err := m.GetDecl(y)
	equationMust(t, err)
	equationMust(t, c.Insert(xd, []ast.ASTHandle{fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "8"))}))
	equationMust(t, c.Insert(yd, []ast.ASTHandle{fixDLApp(t, m, 2, y, fixDLNumber(t, m, false, "4"))}))
	md, err := model.NewModel(m)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, md.Close()) })
	var ref tactic.ModelRef
	equationMust(t, ref.Set(md.Core()))
	t.Cleanup(func() { equationMust(t, ref.Close()) })
	m.ResourceLimit().Cancel()
	err = c.ApplyModel(&ref, 0)
	m.ResourceLimit().ResetCancel()
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatal("missing per-clause cancellation", err)
	}
	if v, err := md.Core().ConstantInterpretation(yd); err != nil || !v.IsNull() {
		t.Fatal("canceled before first clause", v, err)
	}
	equationMust(t, m.ResourceLimit().Push(1))
	err = c.ApplyModel(&ref, 0)
	m.ResourceLimit().Pop()
	if err == nil {
		t.Fatal("missing second-clause resource limit")
	}
	if got := fmValue(t, m, md.Core(), y); got != "3" {
		t.Fatal("lost completed reverse prefix", got)
	}
	if v, err := md.Core().ConstantInterpretation(xd); err != nil || !v.IsNull() {
		t.Fatal("published interrupted variable", v, err)
	}
	equationMust(t, c.ApplyModel(&ref, 0))
	if got := fmValue(t, m, md.Core(), x); got != "7" {
		t.Fatal("failed to resume reconstruction", got)
	}
}

func TestFMModelEmptySavedRowDoesNotCheckCancellation(t *testing.T) {
	c, m := fmModelFixture(t)
	x := fixDLConst(t, m, false, "x")
	xd, err := m.GetDecl(x)
	equationMust(t, err)
	equationMust(t, c.Insert(xd, nil))
	md, err := model.NewModel(m)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, md.Close()) })
	var ref tactic.ModelRef
	equationMust(t, ref.Set(md.Core()))
	t.Cleanup(func() { equationMust(t, ref.Close()) })
	before := m.ResourceLimit().Count()
	m.ResourceLimit().Cancel()
	err = c.ApplyModel(&ref, 0)
	m.ResourceLimit().ResetCancel()
	equationMust(t, err)
	if got := fmValue(t, m, md.Core(), x); got != "0" {
		t.Fatal(got)
	}
	if m.ResourceLimit().Count() != before {
		t.Fatal("empty row introduced a checkpoint")
	}
}
