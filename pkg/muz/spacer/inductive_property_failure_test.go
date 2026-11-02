package spacer

// Source: inductive_property::to_model in src/muz/spacer/spacer_manager.cpp.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestSpacerInductivePropertyFunctionAllocationFailure(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 512 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			var budget *arena.Budget
			t.Cleanup(func() {
				if budget != nil && budget.Used() != 0 {
					t.Errorf("inductive model leaked %d budget bytes", budget.Used())
				}
			})
			m, _ := newSpacerTestServices(t)
			budget = m.Budget()
			boolean, err := m.MkBoolSort()
			depthMust(t, err)
			decl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				s, err := m.Symbols.Intern(name)
				depthMust(t, err)
				d, err := m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{})
				depthMust(t, err)
				return d
			}
			variable := decl("failure-x", nil)
			predicate := decl("failure-p", []ast.ASTHandle{boolean})
			nullary := decl("failure-c", nil)
			body, err := m.MkApp(variable, nil)
			depthMust(t, err)
			falsity, err := m.MkFalse()
			depthMust(t, err)
			property, err := NewInductiveProperty(m, []RelationInfo{{Predicate: predicate, Variables: []ast.ASTHandle{variable}, Body: body}, {Predicate: nullary, Body: falsity}})
			depthMust(t, err)
			defer func() { depthMust(t, property.Close()) }()
			refs := m.Stats().LiveReferences
			m.Budget().FailAfter(stop)
			md, failure := property.Model()
			m.Budget().FailAfter(-1)
			if failure != nil {
				failures++
				if md != nil {
					t.Fatal("failed model was published")
				}
				var cleanup memory.ConstructionFailure
				if errors.As(failure, &cleanup) {
					depthMust(t, cleanup.Cleanup())
				}
				var resource memory.Failure
				if !errors.Is(failure, arena.ErrInjectedFailure) && (!errors.As(failure, &resource) || resource.Kind != memory.FailureMemout) {
					t.Fatal("unexpected failure", failure)
				}
			} else {
				success = true
				depthMust(t, md.Close())
			}
			if got := m.Stats().LiveReferences; got != refs {
				t.Fatal("failed construction lost references", refs, got, failure)
			}
			md, err = property.Model()
			depthMust(t, err)
			function, err := md.Core().FunctionInterpretation(predicate)
			depthMust(t, err)
			value, err := function.Else()
			depthMust(t, err)
			want, err := m.MkVar(0, boolean)
			depthMust(t, err)
			if value != want {
				t.Fatal("retry did not abstract predicate argument", value, want)
			}
			value, err = md.Core().ConstantInterpretation(nullary)
			depthMust(t, err)
			if value != falsity {
				t.Fatal("retry lost nullary interpretation")
			}
			depthMust(t, md.Close())
			if got := m.Stats().LiveReferences; got != refs {
				t.Fatal("retry leaked references", refs, got)
			}
		})
		if t.Failed() {
			break
		}
	}
	if failures == 0 || !success {
		t.Fatal("incomplete function-construction frontier", failures, success)
	}
}
