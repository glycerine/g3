package qe

// Source: src/muz/spacer/spacer_util.cpp:92-103,1213-1261.

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestLegacyModelTermCandidateCatchesTypedEvaluatorException(t *testing.T) {
	for _, multiple := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "multiple"}[multiple], func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			p, err := NewLegacyProjection(m, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, p.Close()) }()
			x, k := f.constant("limited-mbqi-x", f.integer), f.constant("limited-mbqi-k", f.integer)
			zero, one, two := f.number("0"), f.number("1"), f.number("2")
			index := f.own(m.MkArithmeticAdd(k, one))
			sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
			a := f.constant("limited-mbqi-array", sort)
			arrays, err := m.ArrayFamilyID()
			fmMust(t, err)
			selected := f.own(m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, index}))
			square := f.own(m.MkArithmeticMul(x, x))
			positive := f.own(m.MkArithmeticGT(square, zero))
			equality := f.own(m.MkEq(selected, zero))
			forms := []ast.ASTHandle{positive, equality}
			if multiple {
				later := f.own(m.MkArithmeticAdd(k, two))
				selected := f.own(m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, later}))
				forms = append(forms, f.own(m.MkEq(selected, zero)))
			}
			formula := f.own(m.MkAndArray(forms))
			current, err := m.NewRef(formula)
			fmMust(t, err)
			defer func() { fmMust(t, m.DelRef(&current)) }()
			md, err := model.NewModel(m)
			fmMust(t, err)
			defer func() { fmMust(t, md.Close()) }()
			base := f.own(m.MkConstArray(sort, zero))
			for i, v := range []ast.ASTHandle{x, k, a} {
				decl, err := m.GetDecl(v)
				fmMust(t, err)
				fmMust(t, md.Core().RegisterConstant(decl, []ast.ASTHandle{two, one, base}[i]))
			}
			local, err := f.r.config.Context.NewValue()
			fmMust(t, err)
			defer func() { fmMust(t, f.r.config.Context.Del(&local)) }()
			key := "max_steps"
			fmMust(t, f.r.config.Context.SetUIntName(&local, &key, 0))
			config := model.ModelEvaluatorConfig{Context: f.r.config.Context, Globals: f.r.config.Globals, Cooperation: f.r.config.Cooperation, Memory: m.NumeralService().RuntimeOwner(), Params: &local}
			before := m.Stats().LiveReferences
			evaluator, err := newLegacyModelEvaluator(md.Core(), config)
			fmMust(t, err)
			fmMust(t, evaluator.SetModelCompletion(false))
			eliminated, cause := p.projectModelTerm(evaluator, current, x)
			fmMust(t, evaluator.Close())
			if multiple {
				if !errors.Is(cause, ast.ErrASTDomain) {
					t.Fatal("interrupted traversal precondition lost", cause)
				}
			} else {
				fmMust(t, cause)
			}
			if eliminated {
				t.Fatal("failed candidate eliminated variable")
			}
			actual, err := m.RefGet(current)
			fmMust(t, err)
			if actual != formula {
				t.Fatal("failed probe changed formula")
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("candidate failure leaked references", got, before)
			}
			config.Params = nil
			evaluator, err = newLegacyModelEvaluator(md.Core(), config)
			fmMust(t, err)
			eliminated, cause = p.projectModelTerm(evaluator, current, x)
			fmMust(t, cause)
			fmMust(t, evaluator.Close())
			if !eliminated {
				t.Fatal("unlimited fresh evaluator did not recover")
			}
			wantSquare := f.own(m.MkArithmeticMul(index, index))
			wantPositive := f.own(m.MkArithmeticGT(wantSquare, zero))
			forms[0] = wantPositive
			want := f.own(m.MkAndArray(forms))
			actual, err = m.RefGet(current)
			fmMust(t, err)
			if actual != want {
				t.Fatal("retry did not use first equivalent index")
			}
		})
	}
}

func TestLegacyModelTermNullValueFormatting(t *testing.T) {
	f := newFMFixture(t)
	var out bytes.Buffer
	f.r.manager.Budget().FailAfter(0)
	err := writeExpression(f.r.manager, f.r.config, &out, ast.ASTHandle{})
	f.r.manager.Budget().FailAfter(-1)
	fmMust(t, err)
	if out.String() != "null" {
		t.Fatal("source null expression formatting", out.String())
	}
}
