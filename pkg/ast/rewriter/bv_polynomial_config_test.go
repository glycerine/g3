package rewriter

// Source: src/ast/rewriter/poly_rewriter.h:95-123; src/ast/rewriter/poly_rewriter_def.h:28-45.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func bvPolynomialParamsFixture(t *testing.T) (*uparams.ParamsContext, *uparams.GlobalParams, *arena.Budget) {
	t.Helper()
	b := arena.NewBudget(^uint64(0))
	sy, e := usymbol.NewManager(usymbol.Config{Budget: b, PageSize: 64, DescriptorPageSize: 2})
	astTestMust(t, e)
	q, e := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: b, PageSize: 2}, Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{Budget: b, PageSize: 2}}}})
	astTestMust(t, e)
	c, e := uparams.NewParamsContext(sy, q, uparams.ParamsConfig{Budget: b, PageEntries: 1, Text: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{Budget: b, PageSize: 2}}})
	astTestMust(t, e)
	g, e := uparams.NewGlobalParams(sy, c, uparams.GlobalParamsConfig{Budget: b, Register: func(uparams.GlobalParamsRegistrar) error { return nil }})
	astTestMust(t, e)
	t.Cleanup(func() {
		astTestMust(t, g.Close())
		astTestMust(t, c.Close())
		astTestMust(t, q.Close())
		astTestMust(t, sy.Close())
		if b.Used() != 0 {
			t.Fatal("retained", b.Used())
		}
	})
	return c, g, b
}

func TestBVPolynomialConfigurationDefaultsAndIndependentSetters(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	p, e := NewBVPolynomialConfiguration(c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	s, e := p.State()
	astTestMust(t, e)
	if !s.Flat || !s.ASTOrder || s.SortSums || s.SumOfMonomials || s.HoistMultiplication || s.HoistCoefficients || s.MonomialLimit != ^uint32(0) || p.passes != 2 {
		t.Fatal(s, p.passes)
	}
	local, e := c.NewValue()
	astTestMust(t, e)
	defer func() { astTestMust(t, c.Del(&local)) }()
	k := "som"
	astTestMust(t, c.SetBoolName(&local, &k, true))
	k = "hoist_mul"
	astTestMust(t, c.SetBoolName(&local, &k, true))
	astTestMust(t, p.Update(&local))
	s, e = p.State()
	astTestMust(t, e)
	if !s.SumOfMonomials || s.HoistMultiplication {
		t.Fatal(s)
	}
	astTestMust(t, p.SetFlat(false))
	astTestMust(t, p.SetSortSums(true))
	s, e = p.State()
	astTestMust(t, e)
	if s.Flat || !s.SumOfMonomials || !s.SortSums {
		t.Fatal("set_flat must not normalize som", s)
	}
	k = "flat"
	astTestMust(t, c.SetBoolName(&local, &k, false))
	astTestMust(t, p.Update(&local))
	s, e = p.State()
	astTestMust(t, e)
	if s.SumOfMonomials || !s.HoistMultiplication || !s.SortSums {
		t.Fatal("update constraints or retained setter", s)
	}
}

func TestBVPolynomialConfigurationTypedLocalModuleFallback(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	module, e := g.SymbolsField.Intern("rewriter")
	astTestMust(t, e)
	cell, e := g.GetParams(module)
	astTestMust(t, e)
	for _, k := range []string{"som", "hoist_mul", "hoist_cmul", "arith_ineq_lhs", "sort_sums"} {
		key := k
		astTestMust(t, c.SetBoolName(cell, &key, true))
	}
	k := "som_blowup"
	astTestMust(t, c.SetUIntName(cell, &k, 17))
	local, e := c.NewValue()
	astTestMust(t, e)
	defer func() { astTestMust(t, c.Del(&local)) }()
	k = "som"
	astTestMust(t, c.SetBoolName(&local, &k, false))
	k = "flat"
	astTestMust(t, c.SetUIntName(&local, &k, 0)) // wrong type falls through to literal true
	k = "som_blowup"
	astTestMust(t, c.SetBoolName(&local, &k, false)) // wrong type falls through to module
	p, e := NewBVPolynomialConfiguration(c, g, &local)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	s, e := p.State()
	astTestMust(t, e)
	if !s.Flat || s.SumOfMonomials || !s.HoistMultiplication || !s.HoistCoefficients || s.ASTOrder || s.SortSums || s.MonomialLimit != 17 {
		t.Fatal(s)
	}
	if c.Stats().References != 2 {
		t.Fatal("snapshots must retire after both constructor updates", c.Stats())
	}
	flat, add, e := p.AdditionOptions()
	astTestMust(t, e)
	if !flat || add.ASTOrder || !add.HoistMultiplication || !add.HoistCoefficients || add.SortSums {
		t.Fatal(flat, add)
	}
	flat, mul, e := p.MultiplicationOptions()
	astTestMust(t, e)
	if !flat || mul.ASTOrder || mul.SumOfMonomials || !mul.HoistCoefficients || mul.MonomialLimit != 17 {
		t.Fatal(flat, mul)
	}
}

func TestBVPolynomialConfigurationEveryColdReservationAndStoppedPrefix(t *testing.T) {
	succeeded := false
	for ordinal := int64(0); ordinal < 128; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			c, g, b := bvPolynomialParamsFixture(t)
			baseline := b.Used()
			b.FailAfter(ordinal)
			p, e := NewBVPolynomialConfiguration(c, g, nil)
			b.FailAfter(-1)
			if e == nil {
				succeeded = true
				if p.passes != 2 || p.placed[0] || p.placed[1] || p.placed[2] {
					t.Fatal("incomplete constructor", p)
				}
			} else {
				if !errors.Is(e, arena.ErrInjectedFailure) {
					t.Fatal(e)
				}
				if p != nil {
					if !p.stopped || p.cause != e {
						t.Fatal("lost failure prefix")
					}
					old := p.state
					if _, err := p.State(); err != e {
						t.Fatal(err)
					}
					if err := p.SetFlat(false); err != e {
						t.Fatal(err)
					}
					if err := p.Update(&p.snapshots[2]); err != e {
						t.Fatal(err)
					}
					if old != p.state {
						t.Fatal("retry changed prefix")
					}
				}
			}
			if p != nil {
				copy := *p
				if e := copy.Close(); !errors.Is(e, uparams.ErrParamsOwner) {
					t.Fatal("copied owner", e)
				}
				cause := p.cause
				astTestMust(t, p.Close())
				astTestMust(t, p.Close())
				if p.cause != cause {
					t.Fatal("cleanup changed history")
				}
			}
			// Interned keys legitimately remain in the borrowed symbol manager.
			if b.Used() < baseline {
				t.Fatal("released borrowed storage")
			}
		})
		if succeeded {
			break
		}
	}
	if !succeeded {
		t.Fatal("never reached success")
	}
}

func TestBVPolynomialConfigurationFeedsMultiplicationCaller(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	p, e := NewBVPolynomialConfiguration(c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	m, _ := bvAddFixture(t)
	xs := bvMergeVars(t, m, 2)
	two := bvAddNum(t, m, 2, 8)
	three := bvAddNum(t, m, 3, 8)
	nested := bvMergeMul(t, m, two, bvMergeMul(t, m, three, xs[0]))
	out, e := m.NewRef(xs[1])
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	core, e := NewBVPolynomialMultiplicationCore(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, core.Close()) }()
	flat, opts, e := p.MultiplicationOptions()
	astTestMust(t, e)
	st, e := core.RewriteApplication(ast.SliceArguments([]ast.ASTHandle{nested, xs[1]}), out, flat, opts)
	astTestMust(t, e)
	if st != RewriteDone {
		t.Fatal(st)
	}
	got, e := m.RefGet(out)
	astTestMust(t, e)
	want := bvMergeMul(t, m, bvAddNum(t, m, 6, 8), bvMergeMul(t, m, xs[0], xs[1]))
	if got != want {
		t.Fatal("configured caller", got, want)
	}
}

func TestBVPolynomialConfigurationWarmUpdatesRetireModuleReferences(t *testing.T) {
	c, g, b := bvPolynomialParamsFixture(t)
	module, e := g.SymbolsField.Intern("rewriter")
	astTestMust(t, e)
	cell, e := g.GetParams(module)
	astTestMust(t, e)
	for _, k := range []string{"som", "hoist_mul"} {
		key := k
		astTestMust(t, c.SetBoolName(cell, &key, true))
	}
	local, e := c.NewValue()
	astTestMust(t, e)
	defer func() { astTestMust(t, c.Del(&local)) }()
	b.FailAfter(1) // only the configuration header; typed getters allocate nothing
	p, e := NewBVPolynomialConfiguration(c, g, &local)
	b.FailAfter(-1)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	if p.passes != 2 || c.Stats().References != 1 {
		t.Fatal("constructor snapshot leak", p.passes, c.Stats())
	}
	b.FailAfter(0)
	e = p.Update(&local)
	b.FailAfter(-1)
	astTestMust(t, e)
	if p.passes != 3 || c.Stats().References != 1 || p.placed[0] || p.placed[1] {
		t.Fatal("update snapshot leak", p.passes, c.Stats())
	}
	s, e := p.State()
	astTestMust(t, e)
	if !s.SumOfMonomials || s.HoistMultiplication {
		t.Fatal(s)
	}
}

func TestBVPolynomialConfigurationDescriptionsAndAdditionCaller(t *testing.T) {
	c, g, b := bvPolynomialParamsFixture(t)
	d, e := uparams.NewParamDescrs(g.SymbolsField, uparams.ParamDescrsConfig{Budget: b})
	astTestMust(t, e)
	defer func() { astTestMust(t, d.Close()) }()
	astTestMust(t, BVPolynomialParameterDescriptions(d))
	for _, key := range []string{"flat", "som", "som_blowup", "hoist_mul", "hoist_cmul"} {
		yes, e := d.ContainsName(key)
		astTestMust(t, e)
		if !yes {
			t.Fatal(key)
		}
	}
	for _, key := range []string{"arith_ineq_lhs", "sort_sums"} {
		yes, e := d.ContainsName(key)
		astTestMust(t, e)
		if yes {
			t.Fatal("poly collector included arithmetic descriptor", key)
		}
	}
	p, e := NewBVPolynomialConfiguration(c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	astTestMust(t, p.SetSortSums(true))
	m, _ := bvAddFixture(t)
	xs := bvMergeVars(t, m, 2)
	out, e := m.NewRef(ast.ASTHandle{})
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	core, e := NewBVPolynomialAdditionCore(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, core.Close()) }()
	flat, opts, e := p.AdditionOptions()
	astTestMust(t, e)
	st, e := core.RewriteApplication(ast.SliceArguments([]ast.ASTHandle{xs[1], xs[0]}), out, flat, opts)
	astTestMust(t, e)
	if st != RewriteDone {
		t.Fatal(st)
	}
	node, e := m.RefGet(out)
	astTestMust(t, e)
	first, e := m.GetArg(node, 0)
	astTestMust(t, e)
	if first != xs[0] {
		t.Fatal("configured AST ordering", first, xs[0])
	}
}

func TestBVPolynomialConfigurationRetainsInterruptedSnapshot(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	other, _, _ := bvPolynomialParamsFixture(t)
	p, err := NewBVPolynomialConfiguration(c, g, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, p.Close()) })
	if p.decrements != nil {
		t.Fatal("fresh configuration materialized decrement receipts")
	}

	const index = 3
	p.snapshots[index], err = c.NewValue()
	astTestMust(t, err)
	p.placed[index] = true
	key, err := c.SymbolsField.Intern("snapshot-numeral")
	astTestMust(t, err)
	numerals := c.NumeralsField
	value, err := numerals.NewValue()
	astTestMust(t, err)
	astTestMust(t, c.SetRat(&p.snapshots[index], key, &value))
	astTestMust(t, numerals.Del(&value))

	// The block consumes its last reference before releasing the numeral.
	// A foreign numeral owner refuses that release without touching its value.
	c.NumeralsField = other.NumeralsField
	err = p.Close()
	c.NumeralsField = numerals
	if !errors.Is(err, unumeral.ErrRationalServiceOwner) || !p.placed[index] ||
		p.decrements == nil || p.decrements[index].Invocation() == 0 {
		t.Fatal("missing interrupted decrement", err)
	}
	astTestMust(t, p.Close())
	if p.placed[index] || p.decrements != nil {
		t.Fatal("snapshot remained live after retry")
	}
}
