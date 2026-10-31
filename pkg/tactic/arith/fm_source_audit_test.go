package arith

import (
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestFMTacticSourceMemorySentinel(t *testing.T) {
	fm, _ := fmTacticFixture(t, false)
	if fm.maxMemory != uint64(^uint(0)) {
		t.Fatal("default UINT_MAX must map to SIZE_MAX", fm.maxMemory)
	}
	for _, mb := range []uint32{0, 1, math.MaxUint32 - 1, math.MaxUint32} {
		name := "max_memory"
		equationMust(t, fm.context.SetUIntName(&fm.params, &name, mb))
		equationMust(t, fm.UpdateParams(&fm.params))
		want := uint64(mb) * 1024 * 1024
		if mb == math.MaxUint32 || want > uint64(^uint(0)) {
			want = uint64(^uint(0))
		}
		if fm.maxMemory != want {
			t.Fatal("source megabytes_to_bytes", mb, fm.maxMemory, want)
		}
	}
}
func TestFMConstraintDeletionErasesTodoBeforeIDReuse(t *testing.T) {
	p, m := fmParserFixture(t, false, false)
	x := fixDLConst(t, m, false, "x")
	bound := fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "1"))
	c, registration, matched, err := p.Parse(bound, ast.ASTDependency{})
	equationMust(t, err)
	if !matched || registration != FMRegistrationIndexed || p.Variables.Index.Todo.Len() != 1 {
		t.Fatal("missing indexed input")
	}
	id := c.ID
	equationMust(t, p.store.Delete(c))
	if p.Variables.Index.Todo.Len() != 0 {
		t.Fatal("deleted constraint remained in subsumption todo")
	}
	replacement, err := p.store.New(nil, []uint32{0}, []big.Rat{fmCRat(1, 1)}, big.NewRat(2, 1), false, ast.ASTDependency{})
	equationMust(t, err)
	if replacement.ID != id {
		t.Fatal("source free-list reuse", replacement.ID, id)
	}
	p.Variables.Register(replacement)
	if got := p.Variables.Index.Todo.Pop(); got != replacement {
		t.Fatal("reused ID found stale object", got, replacement)
	}
	// Source reset_constraints also removes every queued indexed constraint.
	p.Variables.Index.Todo.Insert(replacement)
	equationMust(t, p.store.Close())
	if p.Variables.Index.Todo.Len() != 0 {
		t.Fatal("constraint reset left dangling worklist entries")
	}
}
func TestFMResolvePreservesUpperOnlyLiteralOccurrences(t *testing.T) {
	for _, entry := range []struct{ upper, want []int32 }{
		{[]int32{7, 7}, []int32{2, 7, 7}},
		{[]int32{7, -7}, []int32{2, 7, -7}},
		{[]int32{2, 2, 7}, []int32{2, 7}},
	} {
		got, tautology := fmMergeLiterals([]int32{2}, entry.upper)
		if tautology || !reflect.DeepEqual(got, entry.want) {
			t.Fatal("source scratch signs mark only lower literals", got, tautology, entry.want)
		}
	}
	if _, tautology := fmMergeLiterals([]int32{2}, []int32{-2, 7}); !tautology {
		t.Fatal("cross-parent complement not recognized")
	}
}
func TestFMModelConverterIgnoresIndexedArgument(t *testing.T) {
	c, m := fmModelFixture(t)
	x := fixDLConst(t, m, false, "x")
	d, err := m.GetDecl(x)
	equationMust(t, err)
	equationMust(t, c.Insert(d, []ast.ASTHandle{fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4"))}))
	for _, index := range []uint32{0, 1, math.MaxUint32} {
		md, err := model.NewModel(m)
		equationMust(t, err)
		var ref tactic.ModelRef
		equationMust(t, ref.Set(md.Core()))
		equationMust(t, c.ApplyModel(&ref, index))
		if got := fmValue(t, m, md.Core(), x); got != "3" {
			t.Fatal("source unused goal index", index, got)
		}
		equationMust(t, ref.Close())
		equationMust(t, md.Close())
	}
	// Source inherits the no-op unindexed and label operations.
	equationMust(t, c.ApplyUnindexed(nil))
	equationMust(t, c.ApplyLabels(nil, math.MaxUint32))
}
func TestFMResolveIntegerUnitSourceAssertion(t *testing.T) {
	vars := NewFMVariableTable()
	x, y := vars.Add(true, false), vars.Add(true, false)
	lower := fmSubsumptionConstraint(nil, []uint32{x, y}, []int64{-2, 1}, 2, false)
	upper := fmSubsumptionConstraint(nil, []uint32{x, y}, []int64{3, 1}, 3, false)
	_, _, _, _, err := fmResolvePolynomial(vars, lower, upper, x)
	if ast.ASTSourceDebug {
		if !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("source integer elimination requires a unit coefficient", err)
		}
	} else {
		equationMust(t, err)
	}
}
func TestFMModelIntegerStrictSourceIntegralAssertion(t *testing.T) {
	c, m := fmModelFixture(t)
	x, y := fixDLConst(t, m, true, "x"), fixDLConst(t, m, false, "y")
	sum := fixDLApp(t, m, 6, fixDLApp(t, m, 14, x), y)
	nonStrict := fixDLApp(t, m, 3, sum, fixDLNumber(t, m, false, "2"))
	strict, err := m.MkNot(nonStrict)
	equationMust(t, err)
	fixDLPin(t, m, strict)
	xd, err := m.GetDecl(x)
	equationMust(t, err)
	equationMust(t, c.Insert(xd, []ast.ASTHandle{strict}))
	md, err := model.NewModel(m)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, md.Close()) })
	yd, err := m.GetDecl(y)
	equationMust(t, err)
	equationMust(t, md.Core().RegisterConstant(yd, fixDLNumber(t, m, false, "1/3")))
	var ref tactic.ModelRef
	equationMust(t, ref.Set(md.Core()))
	t.Cleanup(func() { equationMust(t, ref.Close()) })
	err = c.ApplyModel(&ref, 0)
	if ast.ASTSourceDebug {
		if !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("source integer strict bound asserts integral residual", err)
		}
	} else {
		equationMust(t, err)
		if got := fmValue(t, m, md.Core(), x); got != "0" {
			t.Fatal("source release subtract-one/floor", got)
		}
	}
}
func TestFMModelOnlyFalseLiteralSourceAssertion(t *testing.T) {
	if !ast.ASTSourceDebug {
		t.Skip("source SASSERT(found) debug boundary")
	}
	c, m := fmModelFixture(t)
	x := fixDLConst(t, m, false, "x")
	d, err := m.GetDecl(x)
	equationMust(t, err)
	p := fmBoolConst(t, m, "p")
	pd, err := m.GetDecl(p)
	equationMust(t, err)
	equationMust(t, c.Insert(d, []ast.ASTHandle{p}))
	md, err := model.NewModel(m)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, md.Close()) })
	no, err := m.MkFalse()
	equationMust(t, err)
	equationMust(t, md.Core().RegisterConstant(pd, no))
	var ref tactic.ModelRef
	equationMust(t, ref.Set(md.Core()))
	t.Cleanup(func() { equationMust(t, ref.Close()) })
	if err = c.ApplyModel(&ref, 0); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("source missing-inequality assertion", err)
	}
}

func TestFMTacticSourceParameterDescriptionOrder(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	descriptions, err := uparams.NewParamDescrs(m.Symbols, uparams.ParamDescrsConfig{Budget: m.Budget()})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, descriptions.Close()) })
	equationMust(t, fm.CollectParamDescrs(descriptions))
	names := []string{"produce_models", "max_memory", "fm_real_only", "fm_occ", "fm_limit", "fm_cutoff1", "fm_cutoff2", "fm_extra"}
	size, err := descriptions.Size()
	equationMust(t, err)
	if int(size) != len(names) {
		t.Fatal("source parameter count", size)
	}
	for i, name := range names {
		symbol, err := descriptions.ParamName(uint32(i))
		equationMust(t, err)
		actual, err := m.Symbols.String(symbol)
		equationMust(t, err)
		if actual != name {
			t.Fatal("source insertion order", i, actual, name)
		}
		kind, err := descriptions.KindName(name)
		equationMust(t, err)
		want := uparams.ParamUInt
		if name == "produce_models" || name == "fm_real_only" || name == "fm_occ" {
			want = uparams.ParamBool
		}
		if kind != want {
			t.Fatal("source parameter kind", name, kind, want)
		}
		if value, present, err := descriptions.DefaultName(name); err != nil || present {
			t.Fatal("source defaults are in descriptions, not separate fields", name, value, present, err)
		}
	}
	text, present, err := descriptions.DescriptionName("max_memory")
	equationMust(t, err)
	if !present || text != "(default: infty) maximum amount of memory in megabytes." {
		t.Fatal(text, present)
	}
}
