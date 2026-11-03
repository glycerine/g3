package fp_test

// Source: dl_context.cpp:463-472,876 flushes pending closed formulas using
// current options before validation, then leaves those clauses normalized.

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPendingClosedQueryFlushUsesCurrentUnboundParameter(t *testing.T) {
	for _, initial := range []bool{false, true} {
		t.Run(map[bool]string{false: "enable-at-query", true: "disable-at-query"}[initial], func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			fpMust(t, err)
			t.Cleanup(func() { fpMust(t, context.Close()) })
			globals, err := context.GlobalParameters()
			fpMust(t, err)
			setting := "false"
			if initial {
				setting = "true"
			}
			fpMust(t, globals.SourceSetName("fixedpoint.xform.fix_unbound_vars", setting))
			m, err := context.Manager()
			fpMust(t, err)
			e, err := fp.NewPropositionalEngineWithServices(m, globals.Context, globals)
			fpMust(t, err)
			t.Cleanup(func() { fpMust(t, e.Close()) })
			fpMust(t, e.SelectEngine("pdr"))
			boolean, err := m.MkBoolSort()
			fpMust(t, err)
			pName, err := m.Symbols.Intern("pending-closed-head")
			fpMust(t, err)
			p, err := m.MkFuncDecl(pName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			fpMust(t, err)
			fpMust(t, e.RegisterPredicate(p))
			opaqueName, err := m.Symbols.Intern("pending-closed-opaque")
			fpMust(t, err)
			opaque, err := m.MkFuncDecl(opaqueName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			fpMust(t, err)
			x, err := m.MkVar(0, boolean)
			fpMust(t, err)
			y, err := m.MkVar(1, boolean)
			fpMust(t, err)
			head, err := m.MkApp(p, []ast.ASTHandle{x})
			fpMust(t, err)
			tail, err := m.MkApp(opaque, []ast.ASTHandle{y})
			fpMust(t, err)
			body, err := m.MkImplies(tail, head)
			fpMust(t, err)
			nameX, err := m.Symbols.Intern("closed-source-x")
			fpMust(t, err)
			nameY, err := m.Symbols.Intern("closed-source-y")
			fpMust(t, err)
			original, err := m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall: true, Sorts: []ast.ASTHandle{boolean, boolean},
				Names: []symbol.Symbol{nameY, nameX}, Body: body, Weight: 7,
			})
			fpMust(t, err)
			const ruleName, bound = "closed-unbound-rule", uint32(11)
			fpMust(t, e.AddRuleWithBound(original, ruleName, bound))
			var roots []ast.ASTRef
			t.Cleanup(func() {
				for i := len(roots) - 1; i >= 0; i-- {
					fpMust(t, m.DelRef(&roots[i]))
				}
			})
			readRule := func() ast.ASTHandle {
				t.Helper()
				formulas, err := e.RuleFormulas()
				fpMust(t, err)
				roots = append(roots, formulas...)
				if len(formulas) != 1 {
					t.Fatal("closed input changed rule count", len(formulas))
				}
				h, err := m.RefGet(formulas[0])
				fpMust(t, err)
				return h
			}
			checkRaw := func() {
				t.Helper()
				formulas, names, bounds, err := e.RawRules()
				fpMust(t, err)
				roots = append(roots, formulas...)
				if len(formulas) != 1 || len(names) != 1 || len(bounds) != 1 || names[0] != ruleName || bounds[0] != bound {
					t.Fatal("query flush changed raw rule metadata", len(formulas), names, bounds)
				}
				h, err := m.RefGet(formulas[0])
				fpMust(t, err)
				if h != original {
					t.Fatal("query flush replaced original raw formula", h, original)
				}
			}
			display := func() string {
				t.Helper()
				print := globals.Context.EmptyValue()
				key := "print_with_variable_declarations"
				fpMust(t, globals.Context.SetBoolName(&print, &key, false))
				defer func() { fpMust(t, globals.Context.Del(&print)) }()
				var out bytes.Buffer
				fpMust(t, e.WriteContextSMT2(&out, nil, &print))
				return out.String()
			}
			local := globals.Context.EmptyValue()
			key := "xform.fix_unbound_vars"
			fpMust(t, globals.Context.SetBoolName(&local, &key, !initial))
			fpMust(t, e.SetQueryParameters(&local))
			fpMust(t, globals.Context.Del(&local))
			if readRule() != original {
				t.Fatal("enumeration flushed a pending closed input")
			}
			pendingText := display()
			for _, text := range []string{"closed-source-x", "closed-source-y", ":weight 7", ":named " + ruleName} {
				if !strings.Contains(pendingText, text) {
					t.Fatal("pending display lost original metadata", text, pendingText)
				}
			}
			checkRaw()
			query, err := m.MkFalse()
			fpMust(t, err)
			rejectQuery := func() {
				t.Helper()
				status, err := e.QueryExpression(query)
				var property *base.RulePropertyError
				if status != lbool.Undef || !errors.Is(err, fp.ErrPropositionalHorn) || !errors.As(err, &property) || property.Kind != base.RulePropertyUninterpreted || property.Symbol != "pending-closed-opaque" {
					t.Fatal("query did not reach the post-flush uninterpreted validator", status, err)
				}
			}
			rejectQuery()
			normalized := readRule()
			if normalized == original {
				t.Fatal("failed query left the input in its pending closed phase")
			}
			formula := normalized
			for {
				kind, err := m.GetKind(formula)
				fpMust(t, err)
				if kind != ast.ASTQuantifier {
					break
				}
				formula, err = m.GetQuantifierBody(formula)
				fpMust(t, err)
			}
			antecedent, err := m.GetArg(formula, 0)
			fpMust(t, err)
			if initial {
				declaration, err := m.GetDecl(antecedent)
				fpMust(t, err)
				if declaration != opaque {
					t.Fatal("query-time disabling did not restore the opaque unbound tail")
				}
			} else {
				op, err := m.GetDeclKind(antecedent)
				fpMust(t, err)
				if op != ast.ASTOpIff {
					t.Fatal("query-time enabling did not wrap the existential tail", op)
				}
				quantified, err := m.GetArg(antecedent, 0)
				fpMust(t, err)
				kind, err := m.GetKind(quantified)
				fpMust(t, err)
				if kind != ast.ASTQuantifier {
					t.Fatal("query-time unbound fix omitted its quantifier", kind)
				}
				forall, err := m.IsForall(quantified)
				fpMust(t, err)
				if forall {
					t.Fatal("query-time unbound fix used a universal tail")
				}
			}
			flushedText := display()
			if strings.Contains(flushedText, "closed-source-") || strings.Contains(flushedText, ":weight 7") || !strings.Contains(flushedText, ":named "+ruleName) {
				t.Fatal("failed-query display did not use normalized rule metadata", flushedText)
			}
			checkRaw()
			// Restore the original global option. Once flushed, neither another
			// failed query nor enumeration may normalize the same input again.
			fpMust(t, e.SetQueryParameters(nil))
			for repeat := 0; repeat < 2; repeat++ {
				rejectQuery()
				if readRule() != normalized || display() != flushedText {
					t.Fatal("repeated query rebuilt a flushed rule under changed options", repeat)
				}
				checkRaw()
			}
		})
	}
}
