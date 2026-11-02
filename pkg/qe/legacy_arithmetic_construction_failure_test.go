package qe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyRealProjectionConstructionFailureOwnership(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 2048 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			p, err := newLegacyArithmeticProjection(m, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, p.Close()) }()
			x := f.constant("real-failure-x", f.real)
			zero := f.own(m.ArithmeticRealNumeralText("0"))
			one := f.own(m.ArithmeticRealNumeralText("1"))
			three := f.own(m.ArithmeticRealNumeralText("3"))
			upper := f.own(m.MkArithmeticLE(x, three))
			lower := f.own(m.MkArithmeticGE(x, zero))
			md, err := model.NewModel(m)
			fmMust(t, err)
			defer func() { fmMust(t, md.Close()) }()
			decl, err := m.GetDecl(x)
			fmMust(t, err)
			fmMust(t, md.Core().RegisterConstant(decl, one))
			before := m.Stats().LiveReferences
			m.Budget().FailAfter(stop)
			result, projected, err := p.projectReal(md.Core(), x, []ast.ASTHandle{upper, lower})
			m.Budget().FailAfter(-1)
			if result != nil {
				fmMust(t, result.Close())
			}
			if err == nil {
				if !projected {
					t.Fatal("successful finite projection was not published")
				}
				succeeded = true
			} else {
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					fmMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected projection failure", err)
				}
				failures++
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("real projection kept temporary roots", got, before)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing real projection failure/success frontier", failures, succeeded)
	}
}

func TestLegacyIntegerProjectionConstructionFailureOwnership(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 2048 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			p, err := newLegacyArithmeticProjection(m, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, p.Close()) }()
			x := f.constant("integer-failure-x", f.integer)
			zero := f.own(m.ArithmeticIntegerNumeralText("0"))
			one := f.own(m.ArithmeticIntegerNumeralText("1"))
			three := f.own(m.ArithmeticIntegerNumeralText("3"))
			upper := f.own(m.MkArithmeticLE(x, three))
			lower := f.own(m.MkArithmeticGE(x, zero))
			md, err := model.NewModel(m)
			fmMust(t, err)
			defer func() { fmMust(t, md.Close()) }()
			decl, err := m.GetDecl(x)
			fmMust(t, err)
			fmMust(t, md.Core().RegisterConstant(decl, one))
			before := m.Stats().LiveReferences
			m.Budget().FailAfter(stop)
			result, projected, err := p.projectInteger(md.Core(), x, []ast.ASTHandle{upper, lower})
			m.Budget().FailAfter(-1)
			if result != nil {
				fmMust(t, result.Close())
			}
			if err == nil {
				if !projected {
					t.Fatal("successful finite projection was not published")
				}
				succeeded = true
			} else {
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					fmMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected projection failure", err)
				}
				failures++
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("integer projection kept temporary roots", got, before)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing integer projection failure/success frontier", failures, succeeded)
	}
}

func TestLegacyArithmeticFormulaConstructionFailureOwnership(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 4096 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			p, err := newLegacyArithmeticProjection(m, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, p.Close()) }()
			x := f.constant("formula-failure-x", f.real)
			y := f.constant("formula-failure-y", f.real)
			zero := f.own(m.ArithmeticRealNumeralText("0"))
			one := f.own(m.ArithmeticRealNumeralText("1"))
			three := f.own(m.ArithmeticRealNumeralText("3"))
			formula := f.own(m.MkAndArray([]ast.ASTHandle{
				f.own(m.MkArithmeticLE(x, three)), f.own(m.MkArithmeticGE(x, zero)),
				f.own(m.MkArithmeticLE(y, three)), f.own(m.MkArithmeticGE(y, zero)),
			}))
			md, err := model.NewModel(m)
			fmMust(t, err)
			defer func() { fmMust(t, md.Close()) }()
			for _, variable := range []ast.ASTHandle{x, y} {
				decl, cause := m.GetDecl(variable)
				fmMust(t, cause)
				fmMust(t, md.Core().RegisterConstant(decl, one))
			}
			mapping, err := ast.NewASTExprMapWithProofs(m, false)
			fmMust(t, err)
			defer func() { fmMust(t, mapping.Close()) }()
			before := m.Stats().LiveReferences
			m.Budget().FailAfter(stop)
			result, err := p.projectFormulaMapped(md.Core(), []ast.ASTHandle{x, y}, formula, mapping)
			m.Budget().FailAfter(-1)
			if result != nil {
				fmMust(t, result.Close())
			}
			if err == nil {
				succeeded = true
			} else {
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					fmMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected formula projection failure", err)
				}
				failures++
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("formula projection kept temporary roots", got, before)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing formula projection failure/success frontier", failures, succeeded)
	}
}

func TestLegacyModFactoringConstructionFailureOwnership(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 4096 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			p, err := newLegacyArithmeticProjection(m, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, p.Close()) }()
			x := f.constant("mod-failure-x", f.integer)
			zero := f.own(m.ArithmeticIntegerNumeralText("0"))
			one := f.own(m.ArithmeticIntegerNumeralText("1"))
			three := f.own(m.ArithmeticIntegerNumeralText("3"))
			five := f.own(m.ArithmeticIntegerNumeralText("5"))
			seven := f.own(m.ArithmeticIntegerNumeralText("7"))
			inner := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{x, three}))
			outer := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{f.own(m.MkArithmeticAdd(inner, one)), five}))
			formula := f.own(m.MkAndArray([]ast.ASTHandle{f.own(m.MkArithmeticGE(inner, zero)), f.own(m.MkArithmeticGE(outer, zero))}))
			md, err := model.NewModel(m)
			fmMust(t, err)
			defer func() { fmMust(t, md.Close()) }()
			decl, err := m.GetDecl(x)
			fmMust(t, err)
			fmMust(t, md.Core().RegisterConstant(decl, seven))
			before := m.Stats().LiveReferences
			m.Budget().FailAfter(stop)
			result, err := p.factorModTerms(md.Core(), formula)
			m.Budget().FailAfter(-1)
			if result != nil {
				fmMust(t, result.Close())
			}
			if err == nil {
				succeeded = true
			} else {
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					fmMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected mod factoring failure", err)
				}
				failures++
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("mod factoring kept temporary roots", got, before)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing mod factoring failure/success frontier", failures, succeeded)
	}
}
