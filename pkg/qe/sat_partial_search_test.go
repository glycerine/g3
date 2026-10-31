package qe

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSATPartialSearchBooleanQuantifiers(t *testing.T) {
	for _, strategy := range []uint32{0, 1, 2, 3} {
		for _, tautology := range []bool{false, true} {
			t.Run(fmt.Sprintf("strategy=%d/tautology=%t", strategy, tautology), func(t *testing.T) {
				f := qsatKernelFixture(t)
				m := f.r.manager
				v := f.own(m.MkVar(0, f.boolean))
				body := v
				if tautology {
					notV := f.own(m.MkNot(v))
					body = f.own(m.MkOr(v, notV))
				}
				formula := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
				alt, err := extractSATAlternation(m, f.r.config, formula)
				fmMust(t, err)
				defer func() { fmMust(t, alt.Close()) }()
				options := newSATTacticOptions()
				options.projection = false
				options.strongSimplify = false
				options.extrapolate = strategy
				search, err := newSATPartialSearch(m, f.r.config, alt, options)
				fmMust(t, err)
				defer func() { fmMust(t, search.Close()) }()
				truth := f.own(m.MkTrue())
				result, err := search.qt(0, truth)
				fmMust(t, err)
				defer func() { fmMust(t, result.Close()) }()
				h, err := m.RefGet(result.formula)
				fmMust(t, err)
				if m.IsFalse(h) == tautology {
					t.Fatal("wrong quantified satisfiability result")
				}
				if tautology && result.model == nil {
					t.Fatal("satisfiable result lost model")
				}
			})
		}
	}
}

func TestSATPartialSearchAlternatingQuantifiers(t *testing.T) {
	for _, outerForall := range []bool{false, true} {
		t.Run(fmt.Sprintf("outer-forall=%t", outerForall), func(t *testing.T) {
			f := qsatKernelFixture(t)
			m := f.r.manager
			x := f.own(m.MkVar(1, f.boolean))
			y := f.own(m.MkVar(0, f.boolean))
			body := f.own(m.MkEq(x, y))
			inner := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: !outerForall, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(1)}, Body: body}))
			formula := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: outerForall, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: inner}))
			alt, err := extractSATAlternation(m, f.r.config, formula)
			fmMust(t, err)
			defer func() { fmMust(t, alt.Close()) }()
			options := newSATTacticOptions()
			options.projection = false
			options.strongSimplify = false
			search, err := newSATPartialSearch(m, f.r.config, alt, options)
			fmMust(t, err)
			defer func() { fmMust(t, search.Close()) }()
			truth := f.own(m.MkTrue())
			result, err := search.qt(0, truth)
			fmMust(t, err)
			defer func() { fmMust(t, result.Close()) }()
			h, err := m.RefGet(result.formula)
			fmMust(t, err)
			if m.IsFalse(h) == outerForall {
				t.Fatal("wrong alternating quantifier result")
			}
		})
	}
}

func TestSATPartialSearchOptionsAndCancellation(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	formula := f.own(m.MkTrue())
	alt, err := extractSATAlternation(m, f.r.config, formula)
	fmMust(t, err)
	defer func() { fmMust(t, alt.Close()) }()
	options := newSATTacticOptions()
	if _, err = newSATPartialSearch(m, f.r.config, alt, options); err == nil {
		t.Fatal("unsupported full projection accepted")
	}
	options.projection = false
	if _, err = newSATPartialSearch(m, f.r.config, alt, options); err == nil {
		t.Fatal("unsupported context simplification accepted")
	}
	options.strongSimplify = false
	search, err := newSATPartialSearch(m, f.r.config, alt, options)
	fmMust(t, err)
	defer func() { fmMust(t, search.Close()) }()
	m.ResourceLimit().Cancel()
	_, err = search.qt(0, formula)
	m.ResourceLimit().ResetCancel()
	if err == nil {
		t.Fatal("canceled search succeeded")
	}
	result, err := search.qt(0, formula)
	fmMust(t, err)
	fmMust(t, result.Close())
}
