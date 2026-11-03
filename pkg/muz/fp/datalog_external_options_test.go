package fp_test

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestFixedpointExternalRuleExecutionOptions(t *testing.T) {
	for _, test := range []struct {
		name      string
		parameter string
		wantWiden bool
		wantAll   bool
	}{
		{"recursive widening", "datalog.compile_with_widening", true, false},
		{"all or nothing", "datalog.all_or_nothing_deltas", false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			fpMust(t, err)
			manager, err := context.Manager()
			fpMust(t, err)
			globals, err := context.GlobalParameters()
			fpMust(t, err)
			engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
			fpMust(t, err)

			nonempty := make(map[ast.ASTHandle]bool)
			stores, clones := 0, 0
			unionOne, unionTwo, widenTwo := 0, 0, 0
			fpMust(t, engine.SetExternalRelationCallbacks(rel.ExternalRelationCallbacks{
				ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
					kind, cause := manager.GetDeclKind(declaration)
					if cause != nil {
						return cause
					}
					switch kind {
					case ast.DatalogOpEmpty:
						nonempty[outputs[0]] = false
					case ast.DatalogOpStore:
						stores++
						nonempty[outputs[0]] = true
					case ast.DatalogOpClone:
						clones++
						nonempty[outputs[0]] = nonempty[arguments[0]]
					case ast.DatalogOpUnion, ast.DatalogOpWiden:
						if kind == ast.DatalogOpUnion {
							if len(outputs) == 1 {
								unionOne++
							} else if len(outputs) == 2 {
								unionTwo++
							} else {
								t.Fatalf("unexpected external UNION output count %d", len(outputs))
							}
						} else {
							if len(outputs) != 2 {
								t.Fatalf("unexpected external WIDEN output count %d", len(outputs))
							}
							widenTwo++
						}
						added := nonempty[arguments[1]] && !nonempty[outputs[0]]
						nonempty[outputs[0]] = nonempty[outputs[0]] || nonempty[arguments[1]]
						if len(outputs) == 2 {
							nonempty[outputs[1]] = added
						}
					default:
						t.Fatalf("unexpected external option assignment operation %d", kind)
					}
					return nil
				},
				Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
					kind, cause := manager.GetDeclKind(declaration)
					if cause != nil {
						return ast.ASTHandle{}, cause
					}
					if kind != ast.DatalogOpIsEmpty || len(arguments) != 1 {
						return ast.ASTHandle{}, fmt.Errorf("unexpected external option reduction %d", kind)
					}
					return manager.MkBoolVal(!nonempty[arguments[0]])
				},
			}))

			boolean, err := manager.MkBoolSort()
			fpMust(t, err)
			name, err := manager.Symbols.Intern("fixedpoint-external-option-" + test.name)
			fpMust(t, err)
			predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			fpMust(t, err)
			fpMust(t, engine.RegisterPredicate(predicate))
			fpMust(t, engine.SetPredicateRepresentation(predicate, []string{"external_relation"}))
			name, err = manager.Symbols.Intern("fixedpoint-external-option-seed-" + test.name)
			fpMust(t, err)
			seed, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			fpMust(t, err)
			fpMust(t, engine.RegisterPredicate(seed))
			fpMust(t, engine.SetPredicateRepresentation(seed, []string{"external_relation"}))
			truth, err := manager.MkTrue()
			fpMust(t, err)
			fact, err := manager.MkApp(seed, []ast.ASTHandle{truth})
			fpMust(t, err)
			fpMust(t, engine.AddRule(fact, "external-option-seed"))
			x, err := manager.MkVar(0, boolean)
			fpMust(t, err)
			seedBody, err := manager.MkApp(seed, []ast.ASTHandle{x})
			fpMust(t, err)
			seedHead, err := manager.MkApp(predicate, []ast.ASTHandle{x})
			fpMust(t, err)
			lower, err := manager.MkImplies(seedBody, seedHead)
			fpMust(t, err)
			fpMust(t, engine.AddRule(lower, "external-option-lower"))
			body, err := manager.MkApp(predicate, []ast.ASTHandle{x})
			fpMust(t, err)
			head, err := manager.MkApp(predicate, []ast.ASTHandle{x})
			fpMust(t, err)
			recursive, err := manager.MkImplies(body, head)
			fpMust(t, err)
			fpMust(t, engine.AddRule(recursive, "external-option-recursive"))

			local := globals.Context.EmptyValue()
			key := test.parameter
			fpMust(t, globals.Context.SetBoolName(&local, &key, true))
			fpMust(t, engine.SetQueryParameters(&local))
			fpMust(t, globals.Context.Del(&local))

			status, cause := engine.QueryPredicate(predicate)
			fpMust(t, cause)
			if status != lbool.True || stores != 1 || clones == 0 {
				t.Fatal("external execution option did not saturate", status, stores, clones, unionOne, unionTwo, widenTwo)
			}
			if test.wantWiden && (unionOne != 1 || unionTwo != 0 || widenTwo != 1) {
				t.Fatal("widening option did not reach recursive external callback", unionOne, unionTwo, widenTwo)
			}
			if test.wantAll && (unionOne != 1 || unionTwo != 2 || widenTwo != 0) {
				t.Fatal("all-or-nothing option did not rerun the lower-only rule", unionOne, unionTwo, widenTwo)
			}

			fpMust(t, engine.Close())
			fpMust(t, context.Close())
		})
	}
}
