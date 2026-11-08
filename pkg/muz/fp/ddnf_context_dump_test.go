package fp

// Source: ddnf.cpp:494-504 creates a separate datalog context with default
// local parameters; dump_rules:673-676 calls its full context display_smt2.

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestDDNFContextDumpUsesInnerModuleProfiles(t *testing.T) {
	profiles := []struct {
		name                                  string
		configure, extensions, variables, low bool
	}{
		{"defaults", false, true, true, false},
		{"module-horn", true, false, false, true},
		{"module-quantified", true, true, false, false},
		{"module-low-level", true, true, true, true},
	}
	for _, profile := range profiles {
		t.Run(profile.name, func(t *testing.T) {
			m, globals := newPDRLifecycleServices(t)
			keys := []string{"print_fixedpoint_extensions", "print_with_variable_declarations", "print_low_level_smt2"}
			values := []bool{profile.extensions, profile.variables, profile.low}
			if profile.configure {
				for i, key := range keys {
					fragmentMust(t, globals.SourceSetName("fixedpoint."+key, strconv.FormatBool(values[i])))
				}
			}
			e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
			fragmentMust(t, err)
			f := &fpRuleNameFixture{t: t, m: m, e: e}
			var refs []ast.ASTRef
			t.Cleanup(func() {
				fragmentMust(t, e.Close())
				fragmentMust(t, closeASTRefs(m, refs))
				fragmentMust(t, closeASTRefs(m, f.roots))
			})
			fragmentMust(t, e.SelectEngine("ddnf"))
			var output bytes.Buffer
			fragmentMust(t, e.SetDDNFOutput(&output))
			bitVector := f.own(m.BitVectorSort(4))
			x := f.own(m.MkVar(0, bitVector))
			_, p := f.predicate("ddnf-dump-P", x)
			qDecl, q := f.predicate("ddnf-dump-Q", x)
			value, err := m.NumeralService().NewValue()
			fragmentMust(t, err)
			fragmentMust(t, m.NumeralService().SetUint64(&value, 1))
			one := f.own(m.BitVectorNumeral(&value, 4))
			fragmentMust(t, m.NumeralService().Del(&value))
			first := f.own(m.MkImplies(f.own(m.MkEq(x, one)), p))
			second := f.own(m.MkImplies(p, q))
			empty, err := m.Symbols.Intern("")
			fragmentMust(t, err)
			labels := []symbol.Symbol{empty, symbol.Numerical(7)}
			fragmentMust(t, e.addRuleWithSymbolEager(first, labels[0]))
			fragmentMust(t, e.addRuleWithSymbolEager(second, labels[1]))
			boolean := f.own(m.MkBoolSort())
			backgroundName, err := m.Symbols.Intern("outer-ddnf-background")
			fragmentMust(t, err)
			backgroundDecl := f.own(m.MkFuncDecl(backgroundName, nil, boolean, ast.ASTFuncDeclInfo{}))
			background := f.own(m.MkApp(backgroundDecl, nil))
			fragmentMust(t, e.SetBackground(background))
			fragmentMust(t, e.AssertBackground(background))
			local := globals.Context.EmptyValue()
			for i, key := range keys {
				fragmentMust(t, globals.Context.SetBoolName(&local, &key, !values[i]))
			}
			fragmentMust(t, e.SetQueryParameters(&local))
			fragmentMust(t, globals.Context.Del(&local))
			before, names, err := e.NamedRulesWithSymbols()
			refs = append(refs, before...)
			fragmentMust(t, err)
			if len(before) != 2 || !reflect.DeepEqual(names, labels) {
				t.Fatal("original typed rules", len(before), names)
			}
			persistent := e.set
			status, err := e.QueryPredicate(qDecl)
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("DDNF unexpectedly executed the compiled rules", status)
			}
			dump := output.String()
			for _, want := range []string{"ddnf-dump-P", "ddnf-dump-Q", "(_ BitVec 2)", " :named )", " :named k!7)"} {
				if !strings.Contains(dump, want) {
					t.Fatal("compiled context dump missing source fragment", want, dump)
				}
			}
			if strings.Contains(dump, "outer-ddnf-background") || strings.Count(dump, " :named ") != 2 {
				t.Fatal("inner context inherited outer background or changed rule names", dump)
			}
			if profile.extensions {
				for _, want := range []string{"(declare-rel ", "(rule ", "(query "} {
					if !strings.Contains(dump, want) {
						t.Fatal("missing source extension command", want, dump)
					}
				}
				if strings.Contains(dump, "(set-logic HORN)") || strings.Contains(dump, "(check-sat)") || strings.Count(dump, "(rule ") != 2 || strings.Count(dump, "(query ") != 1 {
					t.Fatal("outer print override changed inner extension commands", dump)
				}
				if strings.Contains(dump, "(declare-var ") != profile.variables || strings.Contains(dump, "(forall ") == profile.variables {
					t.Fatal("inner variable-declaration module flag was ignored", dump)
				}
				bridge := "(assert (=> (exists"
				if profile.low {
					bridge = "(assert (let ("
				}
				if !strings.Contains(dump, bridge) {
					t.Fatal("inner low-level module flag was ignored", bridge, dump)
				}
			} else {
				for _, want := range []string{"(set-logic HORN)", "(declare-fun ", "(assert ", "(check-sat)"} {
					if !strings.Contains(dump, want) {
						t.Fatal("missing source HORN command", want, dump)
					}
				}
				for _, unwanted := range []string{"(declare-rel ", "(declare-var ", "(rule ", "(query "} {
					if strings.Contains(dump, unwanted) {
						t.Fatal("outer print override changed inner HORN profile", unwanted, dump)
					}
				}
			}
			after, gotLabels, err := e.NamedRulesWithSymbols()
			refs = append(refs, after...)
			fragmentMust(t, err)
			if e.set != persistent || len(after) != len(before) || !reflect.DeepEqual(gotLabels, labels) {
				t.Fatal("DDNF replaced original rules or typed label identities", gotLabels)
			}
			for i := range before {
				old, err := m.RefGet(before[i])
				fragmentMust(t, err)
				current, err := m.RefGet(after[i])
				fragmentMust(t, err)
				if current != old {
					t.Fatal("DDNF modified original clause", i)
				}
			}
			fragmentMust(t, e.Close())
			for _, ref := range refs {
				_, err := m.RefGet(ref)
				fragmentMust(t, err)
			}
		})
	}
}
