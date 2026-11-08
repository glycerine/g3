package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestAPIFixedpointPendingDisplayPreservesBindersAcrossFlushAndScopes(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	b := f.pin(f.m.MkBoolSort())
	p, err := f.fp.DeclareRelation("pending-p", []ast.ASTHandle{b})
	astTestMust(t, err)
	v := f.pin(f.m.MkVar(0, b))
	app := f.pin(f.m.MkApp(p, []ast.ASTHandle{v}))
	makeRule := func(name string) ast.ASTHandle {
		return f.pin(f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{astTestSymbol(t, f.m, name)}, Body: app, Weight: 7}))
	}
	options, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, options.SetBool("print_with_variable_declarations", false))
	astTestMust(t, f.fp.SetParameters(options))
	astTestMust(t, options.Close())
	astTestMust(t, f.fp.AddRule(makeRule("source-first"), "first"))
	check := func(want, absent string) string {
		// AddRule only queues (dl_context::add_rule); the first reader builds and
		// keeps the Go eager rule view, so prime it before counting references.
		_, err := f.fp.SMT2()
		astTestMust(t, err)
		before := f.m.Stats().LiveReferences
		text, err := f.fp.SMT2()
		astTestMust(t, err)
		if want != "" && !strings.Contains(text, want) || absent != "" && strings.Contains(text, absent) {
			t.Fatalf("want %q absent %q in %s", want, absent, text)
		}
		if f.m.Stats().LiveReferences != before {
			t.Fatal("display leaked references")
		}
		return text
	}
	first := check("source-first", "")
	if !strings.Contains(first, ":weight 7") {
		t.Fatal("pending weight lost", first)
	}
	astTestMust(t, f.fp.Push())
	astTestMust(t, f.fp.AddRule(makeRule("source-scoped"), "scoped"))
	check("source-scoped", "")
	astTestMust(t, f.fp.Pop())
	check("source-first", "source-scoped")
	status, err := f.fp.Query(p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	check("pending-p", "source-first")
	astTestMust(t, f.fp.AddRule(makeRule("source-next"), "next"))
	mixed := check("source-next", "source-first")
	if strings.Count(mixed, "(rule ") != 2 {
		t.Fatal("pending normalized duplicate", mixed)
	}
	status, err = f.fp.Query(p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	check("pending-p", "source-next")
}

func TestAPIFixedpointPendingDisplayExpansionAndUpdateRanges(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.UpdateRule(f.pAtom, "replace"))
	disjunction := f.pin(f.m.MkOrArgs([]ast.ASTHandle{f.pAtom, f.qAtom}))
	rule := f.pin(f.m.MkImplies(disjunction, f.uAtom))
	astTestMust(t, f.fp.AddRule(rule, "expanded"))
	astTestMust(t, f.fp.UpdateRule(f.pAtom, "replace"))
	text, err := f.fp.SMT2()
	astTestMust(t, err)
	if strings.Count(text, "(rule ") != 2 || !strings.Contains(text, "(or api-object-fp-p api-object-fp-q)") {
		t.Fatal("expanded input duplicated or update shifted its origin", text)
	}
	status, err := f.fp.Query(f.u)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	text, err = f.fp.SMT2()
	astTestMust(t, err)
	if strings.Count(text, "(rule ") != 3 {
		t.Fatal("normalized split clauses missing", text)
	}
}

func TestAPIFixedpointDualityQueryKeepsClosedRawDisplay(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	b := f.pin(f.m.MkBoolSort())
	p, err := f.fp.DeclareRelation("duality-pending", []ast.ASTHandle{b})
	astTestMust(t, err)
	v := f.pin(f.m.MkVar(0, b))
	app := f.pin(f.m.MkApp(p, []ast.ASTHandle{v}))
	rule := f.pin(f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{astTestSymbol(t, f.m, "duality-original")}, Body: app}))
	astTestMust(t, f.fp.AddRule(rule, "original"))
	options, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, options.SetSymbol("engine", "duality"))
	astTestMust(t, options.SetBool("print_with_variable_declarations", false))
	astTestMust(t, f.fp.SetParameters(options))
	astTestMust(t, options.Close())
	status, err := f.fp.Query(p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	text, err := f.fp.SMT2()
	astTestMust(t, err)
	if !strings.Contains(text, "duality-original") {
		t.Fatal("ordinary Duality query flushed raw input", text)
	}
}

// context::update_rule searches only m_rule_set. Pending formulas are a
// separate input queue; ordinary Duality consumes that queue directly.
func TestAPIFixedpointUpdateBeforeFlushPreservesPendingInputs(t *testing.T) {
	for _, backend := range []string{"pdr", "duality"} {
		t.Run(backend, func(t *testing.T) {
			f := newAPIFixedpointFixture(t)
			options, err := f.c.NewParameterSet()
			astTestMust(t, err)
			astTestMust(t, options.SetSymbol("engine", backend))
			astTestMust(t, f.fp.SetParameters(options))
			astTestMust(t, options.Close())
			astTestMust(t, f.fp.AddRuleWithBound(f.pAtom, "same-name", 3))
			astTestMust(t, f.fp.UpdateRule(f.qAtom, "same-name"))
			text, err := f.fp.SMT2()
			astTestMust(t, err)
			q := strings.Index(text, "(rule (! api-object-fp-q")
			p := strings.Index(text, "(rule (! api-object-fp-p")
			if q < 0 || p <= q {
				t.Fatal("normalized update must precede pending original", text)
			}
			status, err := f.fp.Query(f.p)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("update discarded pending original", status)
			}
			status, err = f.fp.Query(f.q)
			astTestMust(t, err)
			want := lbool.True
			if backend == "duality" {
				want = lbool.False
			}
			if status != want {
				t.Fatal("wrong normalized/raw engine boundary", status, want)
			}
			if backend == "pdr" {
				if err := f.fp.UpdateRule(f.uAtom, "same-name"); err == nil {
					t.Fatal("flushed duplicate names accepted")
				}
				status, err = f.fp.Query(f.q)
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal("rejected update changed normalized rules", status)
				}
			}
		})
	}
}

func TestAPIFixedpointNormalizedUpdateKeepsSourceSlotBeforePendingRules(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.UpdateRule(f.pAtom, "first-normalized"))
	astTestMust(t, f.fp.UpdateRule(f.qAtom, "second-normalized"))
	astTestMust(t, f.fp.AddRule(f.uAtom, "pending-last"))
	stronger := f.pin(f.m.MkImplies(f.uAtom, f.pAtom))
	astTestMust(t, f.fp.UpdateRule(stronger, "first-normalized"))
	text, err := f.fp.SMT2()
	astTestMust(t, err)
	first := strings.Index(text, ":named first-normalized")
	second := strings.Index(text, ":named second-normalized")
	pending := strings.Index(text, ":named pending-last")
	if first < 0 || second <= first || pending <= second {
		t.Fatal("replacement moved out of source slot", text)
	}
	status, err := f.fp.Query(f.p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("pending fact did not establish strengthened rule", status)
	}
}
