package api

// Source dl_context.cpp::query flushes Duality's pending rules before rule
// validation when quantify_arrays is enabled, even if that validation fails.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestAPIFixedpointFailedDualityQueryPreservesSourceDisplayPhase(t *testing.T) {
	for _, quantifyArrays := range []bool{false, true} {
		t.Run(fmt.Sprintf("quantify-arrays=%v", quantifyArrays), func(t *testing.T) {
			f := newAPIFixedpointFixture(t)
			boolean := f.pin(f.m.MkBoolSort())
			p, err := f.fp.DeclareRelation("failed-phase-P", []ast.ASTHandle{boolean})
			astTestMust(t, err)
			q, err := f.fp.DeclareRelation("failed-phase-Q", []ast.ASTHandle{boolean})
			astTestMust(t, err)
			x := f.pin(f.m.MkVar(0, boolean))
			body := f.pin(f.m.MkImplies(f.pin(f.m.MkNot(f.pin(f.m.MkApp(p, []ast.ASTHandle{x})))), f.pin(f.m.MkApp(q, []ast.ASTHandle{x}))))
			original := f.pin(f.m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall: true, Sorts: []ast.ASTHandle{boolean},
				Names: []symbol.Symbol{astTestSymbol(t, f.m, "failed-original-binder")}, Body: body, Weight: 7,
			}))
			astTestMust(t, f.fp.AddRule(original, "negative-input"))
			options, err := f.c.NewParameterSet()
			astTestMust(t, err)
			astTestMust(t, options.SetSymbol("engine", "duality"))
			astTestMust(t, options.SetBool("xform.quantify_arrays", quantifyArrays))
			astTestMust(t, options.SetBool("print_with_variable_declarations", false))
			astTestMust(t, f.fp.SetParameters(options))
			astTestMust(t, options.Close())
			before, err := f.fp.SMT2()
			astTestMust(t, err)
			if !strings.Contains(before, "failed-original-binder") || !strings.Contains(before, ":weight 7") {
				t.Fatal("pending input lost its original annotations", before)
			}
			held, err := f.fp.Rules()
			astTestMust(t, err)
			defer func() {
				for i := range held {
					astTestMust(t, f.m.DelRef(&held[i]))
				}
			}()
			query := f.pin(f.m.MkApp(q, []ast.ASTHandle{f.pin(f.m.MkTrue())}))
			for run := 0; run < 2; run++ {
				_, err = f.fp.QueryExpression(query)
				if err == nil || !strings.Contains(err.Error(), "negat") {
					t.Fatal("negative Duality rule unexpectedly accepted", err)
				}
				text, err := f.fp.SMT2()
				astTestMust(t, err)
				pending := strings.Contains(text, "failed-original-binder") && strings.Contains(text, ":weight 7")
				if pending == quantifyArrays {
					t.Fatal("failed query left the wrong source display phase", quantifyArrays, text)
				}
				forms, err := f.fp.Rules()
				astTestMust(t, err)
				if len(forms) != 1 || len(held) != 1 {
					t.Fatal("failed query changed rule count", len(forms), len(held))
				}
				current, err := f.m.RefGet(forms[0])
				astTestMust(t, err)
				if (current == original) == quantifyArrays {
					t.Fatal("public rule enumeration exposed the wrong source phase", quantifyArrays)
				}
				astTestMust(t, f.m.DelRef(&forms[0]))
				old, err := f.m.RefGet(held[0])
				astTestMust(t, err)
				if old != original {
					t.Fatal("phase transition mutated an independently owned rule snapshot")
				}
			}
		})
	}
}
