package fp

// Source: tactic/extension_model_converter.cpp, replayed by Horn cone pruning.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestHornConeModelEvaluatesDefinitionsBeforeRegistration(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	m := f.manager
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var refs []ast.ASTRef
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(err)
		r, err := m.NewRef(h)
		must(err)
		refs = append(refs, r)
		return h
	}
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			must(m.DelRef(&refs[i]))
		}
	}()
	yes := pin(m.MkTrue())
	no := pin(m.MkFalse())
	b := pin(m.MkBoolSort())
	v := pin(m.MkVar(0, b))
	name, err := m.Symbols.Intern("cone-evaluated-function")
	must(err)
	d := pin(m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{}))
	cases := []struct {
		name       string
		body, want ast.ASTHandle
	}{
		{"true", pin(m.MkOr(yes, yes)), yes},
		{"false", pin(m.MkAnd(yes, no)), no},
		{"symbolic", pin(m.MkOr(v, no)), v},
		{"completion", pin(m.MkOr(f.applications["q"], no)), no},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			md, err := model.NewModel(m)
			must(err)
			ref, err := m.NewRef(tc.body)
			must(err)
			must(f.engine.applyHornConeModelStep(md.Core(), transforms.ConeModelStep{{Declaration: d, Formula: ref}}))
			fi, err := md.Core().FunctionInterpretation(d)
			must(err)
			body, err := fi.Else()
			must(err)
			if body != tc.want {
				t.Fatal("definition registered without source evaluation", body, tc.want)
			}
			if tc.name == "completion" {
				present, err := md.Core().HasInterpretation(f.declarations["q"])
				must(err)
				if !present {
					t.Fatal("model completion was disabled")
				}
			}
			must(m.DelRef(&ref))
			must(md.Close())
		})
	}
	// Reverse replay must make an earlier definition see the later definition.
	md, err := model.NewModel(m)
	must(err)
	dependent, err := m.NewRef(pin(m.MkOr(f.applications["p"], no)))
	must(err)
	value, err := m.NewRef(yes)
	must(err)
	must(f.engine.applyHornConeModelStep(md.Core(), transforms.ConeModelStep{{Declaration: f.declarations["q"], Formula: dependent}, {Declaration: f.declarations["p"], Formula: value}}))
	result, err := md.Core().ConstantInterpretation(f.declarations["q"])
	must(err)
	if result != yes {
		t.Fatal("later definition was not evaluated before its consumer", result)
	}
	must(m.DelRef(&value))
	must(m.DelRef(&dependent))
	must(md.Close())
}
