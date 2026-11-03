package api

// Sources: dl_context.cpp get_rules_along_trace_as_formulas returns the exact
// rule symbols; api_datalog_spacer.inc renders each using symbol::str().

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestAPIFixedpointTracePreservesRuleSymbolKinds(t *testing.T) {
	for _, object := range []bool{true, false} {
		label := "context"
		if object {
			label = "object"
		}
		t.Run(label, func(t *testing.T) {
			f := newAPIFixedpointFixture(t)
			astTestMust(t, f.c.SetFixedpointEngine("spacer"))
			for _, name := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker", "fixedpoint.xform.slice"} {
				astTestMust(t, f.c.SetBoolOption(name, false))
			}
			declare := f.fp.DeclareRelation
			add := f.fp.AddRuleWithSymbol
			query := f.fp.QueryExpression
			typed := f.fp.RulesAlongTraceWithSymbols
			text := f.fp.RulesAlongTrace
			if !object {
				declare = f.c.DeclareRelation
				add = f.c.AddFixedpointRuleWithSymbol
				query = func(atom ast.ASTHandle) (lbool.LBool, error) {
					decl, err := f.m.GetDecl(atom)
					if err != nil {
						return lbool.Undef, err
					}
					return f.c.QueryFixedpoint(decl)
				}
				typed = f.c.FixedpointRulesAlongTraceWithSymbols
				text = f.c.FixedpointRulesAlongTrace
			}
			var atoms, originals []ast.ASTHandle
			names := apiFixedpointRuleNameSymbols(t, f.m)
			for i, name := range []string{"trace-symbol-p", "trace-symbol-q", "trace-symbol-r", "trace-symbol-s"} {
				decl := f.pin(declare(name, nil))
				atom := f.pin(f.m.MkApp(decl, nil))
				formula := atom
				if i != 0 {
					formula = f.pin(f.m.MkImplies(atoms[i-1], atom))
				}
				atoms = append(atoms, atom)
				originals = append(originals, formula)
				astTestMust(t, add(formula, names[i]))
			}
			if forms, labels, err := typed(); err == nil || len(forms) != 0 || len(labels) != 0 {
				releaseAPIRefs(t, f.m, forms)
				t.Fatal("typed trace available before query", labels, err)
			}
			status, err := query(atoms[3])
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("chain query", status)
			}
			forms, labels, err := typed()
			astTestMust(t, err)
			defer releaseAPIRefs(t, f.m, forms)
			want := []symbol.Symbol{{}, names[3], names[2], names[1], names[0]}
			if len(forms) != len(want) || !reflect.DeepEqual(labels, want) {
				t.Fatalf("trace symbols %#v with %d formulas, want %#v", labels, len(forms), want)
			}
			for i := 1; i < len(forms); i++ {
				formula, err := f.m.RefGet(forms[i])
				astTestMust(t, err)
				if formula != originals[len(originals)-i] {
					t.Fatal("trace formula order", i, formula)
				}
			}
			textForms, rendered, err := text()
			astTestMust(t, err)
			releaseAPIRefs(t, f.m, textForms)
			if !reflect.DeepEqual(rendered, []string{"<null>", "k!7", "k!7", "", "<null>"}) {
				t.Fatal("trace symbol::str rendering", rendered)
			}
			status, err = query(atoms[0])
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("replacement query", status)
			}
			for _, ref := range forms {
				_, err = f.m.RefGet(ref)
				astTestMust(t, err)
			}
			if !reflect.DeepEqual(labels, want) {
				t.Fatal("old trace labels changed after clone replacement")
			}
			astTestMust(t, add(atoms[0], symbol.Numerical(9)))
			if stale, names, err := typed(); err == nil || len(stale) != 0 || len(names) != 0 {
				releaseAPIRefs(t, f.m, stale)
				t.Fatal("typed stale trace after mutation", names, err)
			}
		})
	}
}
