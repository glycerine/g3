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

func TestLegacyModelTermProjectionConstructionFailureOwnership(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 4096 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			p, err := NewLegacyProjection(m, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, p.Close()) }()
			x := f.constant("model-term-failure-x", f.integer)
			k := f.constant("model-term-failure-k", f.integer)
			zero, one, two := f.number("0"), f.number("1"), f.number("2")
			candidate := f.own(m.MkArithmeticAdd(k, one))
			arraySort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
			a := f.constant("model-term-failure-array", arraySort)
			arrays, err := m.ArrayFamilyID()
			fmMust(t, err)
			selected := f.own(m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, candidate}))
			square := f.own(m.MkArithmeticMul(x, x))
			formula := f.own(m.MkAnd(f.own(m.MkArithmeticGT(square, zero)), f.own(m.MkEq(selected, zero))))
			current, err := m.NewRef(formula)
			fmMust(t, err)
			defer func() { fmMust(t, m.DelRef(&current)) }()
			md, err := model.NewModel(m)
			fmMust(t, err)
			defer func() { fmMust(t, md.Close()) }()
			base := f.own(m.MkConstArray(arraySort, zero))
			for index, variable := range []ast.ASTHandle{x, k, a} {
				decl, cause := m.GetDecl(variable)
				fmMust(t, cause)
				fmMust(t, md.Core().RegisterConstant(decl, []ast.ASTHandle{two, one, base}[index]))
			}
			before := m.Stats().LiveReferences
			m.Budget().FailAfter(stop)
			remaining, err := p.projectModelTerms(md.Core(), current, []ast.ASTHandle{x})
			m.Budget().FailAfter(-1)
			if err == nil {
				if len(remaining) != 0 {
					t.Fatal("successful symbolic projection kept variable", remaining)
				}
				succeeded = true
			} else {
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					fmMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected model-term failure", err)
				}
				failures++
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("model-term projection kept temporary roots", got, before)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing model-term failure/success frontier", failures, succeeded)
	}
}
