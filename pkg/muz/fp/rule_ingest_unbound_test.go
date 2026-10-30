package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestPropositionalEngineRuleIngestionHonorsUnboundVariableParameter(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			fpMust(t, err)
			t.Cleanup(func() { fpMust(t, context.Close()) })
			globals, err := context.GlobalParameters()
			fpMust(t, err)
			value := "false"
			if enabled {
				value = "true"
			}
			fpMust(t, globals.SourceSetName("fixedpoint.xform.fix_unbound_vars", value))
			manager, err := context.Manager()
			fpMust(t, err)
			engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
			fpMust(t, err)
			t.Cleanup(func() { fpMust(t, engine.Close()) })
			boolean, err := manager.MkBoolSort()
			fpMust(t, err)
			predicateName, err := manager.Symbols.Intern("ingest-unbound-head")
			fpMust(t, err)
			predicate, err := manager.MkFuncDecl(predicateName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			fpMust(t, err)
			fpMust(t, engine.RegisterPredicate(predicate))
			opaqueName, err := manager.Symbols.Intern("ingest-unbound-opaque")
			fpMust(t, err)
			opaqueDecl, err := manager.MkFuncDecl(opaqueName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			fpMust(t, err)
			x, err := manager.MkVar(0, boolean)
			fpMust(t, err)
			y, err := manager.MkVar(1, boolean)
			fpMust(t, err)
			head, err := manager.MkApp(predicate, []ast.ASTHandle{x})
			fpMust(t, err)
			body, err := manager.MkApp(opaqueDecl, []ast.ASTHandle{y})
			fpMust(t, err)
			formula, err := manager.MkImplies(body, head)
			fpMust(t, err)
			fpMust(t, engine.AddRule(formula, "unbound-ingestion"))
			rules, err := engine.Rules()
			fpMust(t, err)
			t.Cleanup(func() {
				for i := len(rules) - 1; i >= 0; i-- {
					fpMust(t, manager.DelRef(&rules[i]))
				}
			})
			if len(rules) != 1 {
				t.Fatal("rule count", len(rules))
			}
			ruleFormula, err := manager.RefGet(rules[0])
			fpMust(t, err)
			for {
				kind, e := manager.GetKind(ruleFormula)
				if e != nil || kind != ast.ASTQuantifier {
					break
				}
				ruleFormula, err = manager.GetQuantifierBody(ruleFormula)
				fpMust(t, err)
			}
			tail, err := manager.GetArg(ruleFormula, 0)
			fpMust(t, err)
			if !enabled {
				declaration, e := manager.GetDecl(tail)
				if e != nil || declaration != opaqueDecl {
					t.Fatal("disabled option changed residual unbound tail", declaration, e)
				}
				return
			}
			operator, err := manager.GetDeclKind(tail)
			if err != nil || operator != ast.ASTOpIff {
				t.Fatal("enabled option did not wrap existential tail", operator, err)
			}
			quantified, err := manager.GetArg(tail, 0)
			fpMust(t, err)
			if kind, e := manager.GetKind(quantified); e != nil || kind != ast.ASTQuantifier {
				t.Fatal("enabled option did not quantify the unbound variable", kind, e)
			}
			if forall, e := manager.IsForall(quantified); e != nil || forall {
				t.Fatal("enabled option used a universal quantifier", forall, e)
			}
		})
	}
}
