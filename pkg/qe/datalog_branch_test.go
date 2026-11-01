package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestDatalogPluginAssignAndSubstituteBranches(t *testing.T) {
	for _, tc := range []struct {
		name        string
		size        uint64
		branch      uint32
		small, last bool
	}{{"small", 1, 0, true, false}, {"large equality", 4, 0, false, false}, {"large other", 4, 1, false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			name, e := m.Symbols.Intern("branch-domain")
			fmMust(t, e)
			sort := f.own(m.MkDatalogFiniteSort(name, tc.size))
			x, y, z := f.constant("x", sort), f.constant("y", sort), f.constant("z", sort)
			eq1, eq2 := f.own(m.MkEq(x, y)), f.own(m.MkEq(x, z))
			neg := f.own(m.MkNot(eq2))
			formula := f.own(m.MkAnd(eq1, neg))
			contains, e := ast.NewContainsApp(m, x)
			fmMust(t, e)
			defer func() { fmMust(t, contains.Close()) }()
			ctx, e := NewSolverContext(m, []ast.ASTHandle{x})
			fmMust(t, e)
			defer func() { fmMust(t, ctx.Close()) }()
			plugin, e := NewDatalogPlugin(ctx)
			fmMust(t, e)
			fmMust(t, ctx.AddPlugin(plugin))
			ok, e := plugin.Prepare(contains, formula, []ast.ASTHandle{eq1}, []ast.ASTHandle{eq2})
			fmMust(t, e)
			if !ok {
				t.Fatal("prepare")
			}
			service := m.NumeralService()
			branch, e := service.NewValue()
			fmMust(t, e)
			defer func() { fmMust(t, service.Del(&branch)) }()
			fmMust(t, service.SetUnsigned(&branch, tc.branch))
			fmMust(t, plugin.Assign(contains, formula, &branch))
			constraints, e := plugin.TakeConstraints()
			fmMust(t, e)
			var want []ast.ASTHandle
			if tc.small {
				zero := f.own(m.MkDatalogNumeral(0, sort))
				want = []ast.ASTHandle{f.own(m.MkEq(x, zero))}
			} else if tc.last {
				want = []ast.ASTHandle{f.own(m.MkNot(eq1)), f.own(m.MkNot(eq2))}
			} else {
				want = []ast.ASTHandle{eq1}
			}
			if len(constraints) != len(want) {
				t.Fatalf("constraints %d want %d", len(constraints), len(want))
			}
			for i, c := range constraints {
				h, e := m.RefGet(c.Formula)
				fmMust(t, e)
				if !c.UseVariable || h != want[i] {
					t.Fatalf("constraint %d", i)
				}
				fmMust(t, m.DelRef(&constraints[i].Formula))
			}
			ref, e := m.NewRef(formula)
			fmMust(t, e)
			defer func() { fmMust(t, m.DelRef(&ref)) }()
			def, e := m.NewRef(eq1)
			fmMust(t, e)
			defer func() { fmMust(t, m.DelRef(&def)) }()
			fmMust(t, plugin.Substitute(contains, &branch, ref, &def))
			result, e := m.RefGet(ref)
			fmMust(t, e)
			definition, e := m.RefGet(def)
			fmMust(t, e)
			if definition != (ast.ASTHandle{}) {
				t.Fatal("definition not cleared")
			}
			var expected ast.ASTHandle
			if tc.small {
				zero := f.own(m.MkDatalogNumeral(0, sort))
				a := f.own(m.MkEq(zero, y))
				b := f.own(m.MkEq(zero, z))
				expected = f.own(m.MkAnd(a, f.own(m.MkNot(b))))
			} else if tc.last {
				no := f.own(m.MkBoolVal(false))
				expected = f.own(m.MkAnd(no, f.own(m.MkNot(no))))
			} else {
				a := f.own(m.MkEq(y, y))
				b := f.own(m.MkEq(y, z))
				expected = f.own(m.MkAnd(a, f.own(m.MkNot(b))))
			}
			if result != expected {
				t.Fatalf("unexpected substitution: %v want %v", result, expected)
			}
			fmMust(t, service.SetUnsigned(&branch, 99))
			if e = plugin.Assign(contains, formula, &branch); !errors.Is(e, ast.ErrASTDomain) {
				t.Fatal("out of range", e)
			}
		})
	}
}

func TestDatalogPluginOtherBranchPreservesNegativeEquality(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	name, err := m.Symbols.Intern("negative-only-domain")
	fmMust(t, err)
	sort := f.own(m.MkDatalogFiniteSort(name, 4))
	x, value := f.constant("negative-only-x", sort), f.constant("negative-only-value", sort)
	equality := f.own(m.MkEq(x, value))
	disequality := f.own(m.MkNot(equality))
	contains, err := ast.NewContainsApp(m, x)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	ctx, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	defer func() { fmMust(t, ctx.Close()) }()
	plugin, err := NewDatalogPlugin(ctx)
	fmMust(t, err)
	fmMust(t, ctx.AddPlugin(plugin))
	prepared, err := plugin.Prepare(contains, disequality, nil, []ast.ASTHandle{equality})
	fmMust(t, err)
	if !prepared {
		t.Fatal("negative equality was not prepared")
	}
	branch, err := m.NumeralService().NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, m.NumeralService().Del(&branch)) }()
	fmMust(t, m.NumeralService().SetUnsigned(&branch, 0))
	if err = plugin.Assign(contains, disequality, &branch); err != nil {
		t.Fatal(err)
	}
	constraints, err := plugin.TakeConstraints()
	fmMust(t, err)
	if len(constraints) != 1 {
		t.Fatalf("branch constraints=%d, want one", len(constraints))
	}
	constraint, err := m.RefGet(constraints[0].Formula)
	fmMust(t, err)
	if !constraints[0].UseVariable || constraint != disequality {
		t.Fatalf("other branch constraint=%v, want %v", constraint, disequality)
	}
	fmMust(t, m.DelRef(&constraints[0].Formula))
	formula, err := m.NewRef(disequality)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&formula)) }()
	fmMust(t, plugin.Substitute(contains, &branch, formula, nil))
	result, err := m.RefGet(formula)
	fmMust(t, err)
	isNot, arg, err := m.IsNot(result)
	fmMust(t, err)
	if !isNot || !m.IsFalse(arg) {
		t.Fatalf("other branch substituted disequality as %v (negated=%v, argument=%v); want not(false)", result, isNot, arg)
	}
}
