package fp

// Source: dl_mk_rule_inliner.cpp:130 calls rule_manager::fix_unbound_vars,
// whose context option at dl_rule.cpp:652 uses local-over-module parameters.

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestDatalogInlinerFixUnboundVariablesLocalPrecedence(t *testing.T) {
	for _, localFix := range []bool{false, true} {
		t.Run("local="+strconv.FormatBool(localFix), func(t *testing.T) {
			m, globals := newPDRLifecycleServices(t)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(globals.SourceSetName("fixedpoint.xform.fix_unbound_vars", strconv.FormatBool(!localFix)))
			e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
			must(err)
			t.Cleanup(func() { must(e.Close()) })
			must(e.SelectEngine("datalog"))
			local := globals.Context.EmptyValue()
			key := "xform.fix_unbound_vars"
			must(globals.Context.SetBoolName(&local, &key, localFix))
			must(e.SetQueryParameters(&local))
			must(globals.Context.Del(&local))

			boolean, err := m.MkBoolSort()
			must(err)
			declare := func(name string, predicate bool) ast.ASTHandle {
				symbol, err := m.Symbols.Intern("inliner-precedence-" + name)
				must(err)
				declaration, err := m.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				if predicate {
					must(e.RegisterPredicate(declaration))
				}
				return declaration
			}
			middle, goal, opaque := declare("middle", true), declare("goal", true), declare("opaque", false)
			x, err := m.MkVar(0, boolean)
			must(err)
			y, err := m.MkVar(1, boolean)
			must(err)
			app := func(declaration, argument ast.ASTHandle) ast.ASTHandle {
				application, err := m.MkApp(declaration, []ast.ASTHandle{argument})
				must(err)
				return application
			}
			middleApp, goalApp, opaqueApp := app(middle, x), app(goal, x), app(opaque, y)
			// Build raw rules so ingestion cannot fix the residual variable before
			// the inliner's own post-resolution normalization reads the option.
			for _, clause := range []struct{ head, body ast.ASTHandle }{{middleApp, opaqueApp}, {goalApp, middleApp}} {
				rule, err := e.rules.NewRule(clause.head, []ast.ASTHandle{clause.body}, nil, "precedence")
				must(err)
				must(e.set.Add(rule))
				must(rule.Close())
			}
			must(e.set.SetOutputPredicate(goal))
			valid, err := e.set.Finalize()
			must(err)
			if !valid {
				t.Fatal("inliner precedence fixture is not stratified")
			}
			plugin, err := e.datalogInliner(globals.Context, globals)
			must(err)
			t.Cleanup(func() { must(plugin.Close()) })
			// The plugin must keep the copied policy after the query-local owner
			// closes; the remaining module value deliberately disagrees.
			must(e.SetQueryParameters(nil))
			transformed, err := plugin.Transform(e.set)
			must(err)
			if transformed == nil {
				t.Fatal("inliner did not resolve the middle predicate")
			}
			t.Cleanup(func() { must(transformed.Close()) })
			var output *base.Rule
			count, err := transformed.Len()
			must(err)
			for index := 0; index < count; index++ {
				rule, err := transformed.Rule(index)
				must(err)
				declaration, err := rule.HeadDeclaration()
				must(err)
				if declaration == goal {
					if output != nil {
						must(rule.Close())
						t.Fatal("duplicate output rule")
					}
					output = rule
				} else {
					must(rule.Close())
				}
			}
			if output == nil {
				t.Fatal("inlining removed the protected output")
			}
			t.Cleanup(func() { must(output.Close()) })
			if size, err := output.TailSize(); err != nil || size != 1 {
				t.Fatal("resolved tail count", size, err)
			}
			tail, negative, err := output.Tail(0)
			must(err)
			if negative {
				t.Fatal("resolved interpreted tail became negative")
			}
			if !localFix {
				declaration, err := m.GetDecl(tail)
				must(err)
				if declaration != opaque {
					t.Fatal("local false did not preserve the residual unbound tail over module true")
				}
				return
			}
			if operation, err := m.GetDeclKind(tail); err != nil || operation != ast.ASTOpIff {
				t.Fatal("local true did not existentially fix the residual tail over module false", operation, err)
			}
			quantified, err := m.GetArg(tail, 0)
			must(err)
			if kind, err := m.GetKind(quantified); err != nil || kind != ast.ASTQuantifier {
				t.Fatal("fixed tail does not contain a quantifier", kind, err)
			}
			if forall, err := m.IsForall(quantified); err != nil || forall {
				t.Fatal("fixed tail is not existential", forall, err)
			}
		})
	}
}

func TestDatalogInlinerVariableEquivalenceLocalPrecedence(t *testing.T) {
	for _, propagate := range []bool{false, true} {
		t.Run("local="+strconv.FormatBool(propagate), func(t *testing.T) {
			m, globals := newPDRLifecycleServices(t)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(globals.SourceSetName("fixedpoint.xform.tail_simplifier_pve", strconv.FormatBool(!propagate)))
			e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
			must(err)
			t.Cleanup(func() { must(e.Close()) })
			must(e.SelectEngine("datalog"))
			local := globals.Context.EmptyValue()
			key := "xform.tail_simplifier_pve"
			must(globals.Context.SetBoolName(&local, &key, propagate))
			must(e.SetQueryParameters(&local))
			must(globals.Context.Del(&local))

			one, err := m.ArithmeticIntegerNumeralText("1")
			must(err)
			integer, err := m.GetSort(one)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			x, err := m.MkVar(0, integer)
			must(err)
			declare := func(name string) (ast.ASTHandle, ast.ASTHandle) {
				symbol, err := m.Symbols.Intern("inliner-pve-" + name)
				must(err)
				declaration, err := m.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(declaration))
				application, err := m.MkApp(declaration, []ast.ASTHandle{x})
				must(err)
				return declaration, application
			}
			_, middle := declare("middle")
			goal, head := declare("goal")
			equality, err := m.MkEq(x, one)
			must(err)
			// x is bound by each head, so unbound-variable reduction preserves
			// x=1 and only the nested interpreted-tail PVE policy propagates it.
			for _, clause := range []struct{ head, body ast.ASTHandle }{{middle, equality}, {head, middle}} {
				rule, err := e.rules.NewRule(clause.head, []ast.ASTHandle{clause.body}, nil, "pve-precedence")
				must(err)
				must(e.set.Add(rule))
				must(rule.Close())
			}
			must(e.set.SetOutputPredicate(goal))
			valid, err := e.set.Finalize()
			must(err)
			if !valid {
				t.Fatal("PVE precedence fixture is not stratified")
			}
			plugin, err := e.datalogInliner(globals.Context, globals)
			must(err)
			t.Cleanup(func() { must(plugin.Close()) })
			must(e.SetQueryParameters(nil))
			transformed, err := plugin.Transform(e.set)
			must(err)
			if transformed == nil {
				t.Fatal("inliner did not resolve the middle predicate")
			}
			t.Cleanup(func() { must(transformed.Close()) })
			if count, err := transformed.Len(); err != nil || count != 1 {
				t.Fatal("resolved output rule count", count, err)
			}
			output, err := transformed.Rule(0)
			must(err)
			t.Cleanup(func() { must(output.Close()) })
			if declaration, err := output.HeadDeclaration(); err != nil || declaration != goal {
				t.Fatal("resolved output predicate", declaration, err)
			}
			resolvedHead, err := output.Head()
			must(err)
			argument, err := m.GetArg(resolvedHead, 0)
			must(err)
			tails, err := output.TailSize()
			must(err)
			if propagate {
				if argument != one || tails != 0 {
					t.Fatal("local true did not propagate x=1 over module false", argument, tails)
				}
				return
			}
			if argument != x || tails != 1 {
				t.Fatal("local false did not preserve variable head and equality over module true", argument, tails)
			}
			tail, negative, err := output.Tail(0)
			must(err)
			if operation, err := m.GetDeclKind(tail); err != nil || operation != ast.ASTOpEq || negative {
				t.Fatal("disabled propagation lost its positive equality", operation, negative, err)
			}
			left, err := m.GetArg(tail, 0)
			must(err)
			right, err := m.GetArg(tail, 1)
			must(err)
			if !(left == x && right == one || left == one && right == x) {
				t.Fatal("disabled propagation changed x=1", left, right)
			}
		})
	}
}
