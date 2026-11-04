package base_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleFixUnboundQuantifierReferenceFailure(t *testing.T) {
	for _, stop := range []int64{9, 10} {
		for _, free := range []int{7, 11} {
			t.Run(fmt.Sprintf("%d/%d", stop, free), func(t *testing.T) {
				f := newUnboundFixture(t)
				m := f.manager
				boolean, err := m.MkBoolSort()
				ruleMust(t, err)
				x, err := m.MkVar(0, boolean)
				ruleMust(t, err)
				y, err := m.MkVar(1, boolean)
				ruleMust(t, err)
				z, err := m.MkVar(2, boolean)
				ruleMust(t, err)
				head := f.predicate(t, "unbound-binding-head", []ast.ASTHandle{boolean, boolean}, []ast.ASTHandle{y, z})
				name, err := m.Symbols.Intern("unbound-binding-opaque")
				ruleMust(t, err)
				decl, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
				ruleMust(t, err)
				tail, err := m.MkApp(decl, []ast.ASTHandle{x})
				ruleMust(t, err)
				name2, err := m.Symbols.Intern("unbound-binding-opaque-second")
				ruleMust(t, err)
				decl2, err := m.MkFuncDecl(name2, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
				ruleMust(t, err)
				tail2, err := m.MkApp(decl2, []ast.ASTHandle{x})
				ruleMust(t, err)
				rule, err := f.rules.NewRule(head, []ast.ASTHandle{tail, tail2}, nil, "unbound-binding")
				ruleMust(t, err)
				defer func() { ruleMust(t, rule.Close()) }()
				for i := 0; i < 2; i++ {
					r, _, err := f.rules.ReduceUnboundVariables(rule, f.lite)
					ruleMust(t, err)
					ruleMust(t, r.Close())
				}
				var pressure []ast.ASTRef
				for i := 0; i < 64; i++ {
					pin, e := m.NewRef(ast.ASTHandle{})
					ruleMust(t, e)
					pressure = append(pressure, pin)
				}
				defer func() {
					for i := range pressure {
						ruleMust(t, m.DelRef(&pressure[i]))
					}
				}()
				for {
					m.Budget().FailAfter(0)
					pin, cause := m.NewRef(ast.ASTHandle{})
					m.Budget().FailAfter(-1)
					if cause != nil {
						if !errors.Is(cause, arena.ErrInjectedFailure) {
							t.Fatal(cause)
						}
						break
					}
					pressure = append(pressure, pin)
					if len(pressure) > 4096 {
						t.Fatal("reference page did not fill")
					}
				}
				for i := 0; i < free; i++ {
					last := len(pressure) - 1
					ruleMust(t, m.DelRef(&pressure[last]))
					pressure = pressure[:last]
				}
				before := m.Stats()
				m.Budget().FailAfter(stop)
				fixed, _, cause := f.rules.FixUnboundVariables(rule, f.lite, true)
				m.Budget().FailAfter(-1)
				if fixed != nil || !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("expected quantifier reference failure", fixed, cause)
				}

				// QE may release a warmed cache reference on failure, so compare for
				// increases rather than requiring an identical cache population.
				after := m.Stats()
				if after.LiveNodes > before.LiveNodes || after.LiveReferences > before.LiveReferences {
					t.Fatal("failed fix left AST roots", before, after, cause)
				}
				if cause != nil {
					fixed, changed, err := f.rules.FixUnboundVariables(rule, f.lite, true)
					ruleMust(t, err)
					if !changed {
						t.Fatal("retry did not bind the unbound variable")
					}
					ruleMust(t, fixed.Close())
				}
			})
		}
	}
}
