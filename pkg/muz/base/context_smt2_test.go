package base_test

// Source: dl_context.cpp display_smt2, is_query and declare_vars.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestContextSMT2OutputQueriesProfilesAndReplay(t *testing.T) {
	for _, extensions := range []bool{false, true} {
		f := newRuleSetFixture(t, "P", "R")
		set, err := base.NewRuleSet(f.rules)
		ruleMust(t, err)
		fact, err := f.rules.NewRule(f.app["R"], nil, nil, "ruleset-R")
		ruleMust(t, err)
		for _, r := range []*base.Rule{fact, f.rule("P", []string{"R"}, nil)} {
			ruleMust(t, set.Add(r))
			ruleMust(t, r.Close())
		}
		ruleMust(t, set.SetOutputPredicate(f.decl["P"]))
		config := base.DefaultContextSMT2Config()
		config.FixedpointExtensions = extensions
		config.Queries = []ast.ASTHandle{f.app["P"]}
		before := f.manager.Stats().LiveReferences
		var out bytes.Buffer
		ruleMust(t, set.WriteContextSMT2(&out, config))
		if f.manager.Stats().LiveReferences != before {
			t.Fatal("context dump leaked formula references")
		}
		text := out.String()
		if !strings.Contains(text, ":named ruleset-R!") {
			t.Fatal("rule/declaration name collision was not freshened", text)
		}
		if extensions {
			for _, part := range []string{"(declare-rel ruleset-R ())", "(declare-rel ruleset-P ())", "(rule (! ruleset-R :named ruleset-R!))", "(query ruleset-R)", "(query ruleset-P)"} {
				if !strings.Contains(text, part) {
					t.Fatal("extension dump missing", part, text)
				}
			}
			if strings.Contains(text, "set-logic") || strings.Contains(text, "(assert (=> ruleset-R ruleset-P))") {
				t.Fatal("output clause was printed as an ordinary assertion", text)
			}
		} else {
			for _, part := range []string{"(set-logic HORN)", "(assert (! ruleset-R :named ruleset-R!))", "(push)\n(assert (not ruleset-R))\n(check-sat)\n(pop)", "(push)\n(assert (not ruleset-P))\n(check-sat)\n(pop)"} {
				if !strings.Contains(text, part) {
					t.Fatal("plain HORN dump missing", part, text)
				}
			}
		}
		replay, err := cmd_context.NewStandaloneContext()
		ruleMust(t, err)
		replay.PrintSuccess = false
		var result bytes.Buffer
		ruleMust(t, smt2.ParseCommands(replay, strings.NewReader(text), &result))
		want := "unsat\nsat\n"
		if extensions {
			want = "sat\nunsat\n"
		}
		if result.String() != want {
			t.Fatal("context dump replay", result.String(), want, text)
		}
		ruleMust(t, replay.Close())
		ruleMust(t, set.Close())
	}
}

func TestContextSMT2BackgroundPlainHORNRejectionPrefix(t *testing.T) {
	f := newRuleSetFixture(t, "P")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	defer func() { ruleMust(t, set.Close()) }()
	r := f.rule("P", nil, nil)
	ruleMust(t, set.Add(r))
	ruleMust(t, r.Close())
	config := base.ContextSMT2Config{Background: []ast.ASTHandle{f.app["P"]}}
	before := f.manager.Stats().LiveReferences
	var out bytes.Buffer
	err = set.WriteContextSMT2(&out, config)
	if err == nil || err.Error() != "Background axioms cannot be used with SMT-LIB2 HORN format" {
		t.Fatal("source background rejection", err)
	}
	if !strings.HasPrefix(out.String(), "(set-logic HORN)\n") || strings.Contains(out.String(), "(assert ") {
		t.Fatal("wrong rejection prefix", out.String())
	}
	if f.manager.Stats().LiveReferences != before {
		t.Fatal("background rejection leaked formulas")
	}
	config = base.DefaultContextSMT2Config()
	config.Background = []ast.ASTHandle{f.app["P"]}
	out.Reset()
	ruleMust(t, set.WriteContextSMT2(&out, config))
	if !strings.Contains(out.String(), "(assert ruleset-P)\n") {
		t.Fatal("extension background omitted", out.String())
	}
}

func TestContextSMT2VariableDeclarationAndQuantifiedQueryProfiles(t *testing.T) {
	for _, declare := range []bool{false, true} {
		for _, low := range []bool{false, true} {
			f := newRuleSetFixture(t)
			m := f.manager
			boolean, err := m.MkBoolSort()
			ruleMust(t, err)
			integerZero, err := m.ArithmeticIntegerNumeralText("0")
			ruleMust(t, err)
			integer, err := m.GetSort(integerZero)
			ruleMust(t, err)
			var pins []ast.ASTRef
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				ruleMust(t, err)
				r, e := m.NewRef(h)
				ruleMust(t, e)
				pins = append(pins, r)
				return h
			}
			pName, err := m.Symbols.Intern("P")
			ruleMust(t, err)
			p := own(m.MkFuncDecl(pName, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{}))
			ruleMust(t, f.rules.RegisterPredicate(p))
			qName, err := m.Symbols.Intern("Q")
			ruleMust(t, err)
			q := own(m.MkFuncDecl(qName, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{}))
			ruleMust(t, f.rules.RegisterPredicate(q))
			x := own(m.MkVar(0, integer))
			px, qx := own(m.MkApp(p, []ast.ASTHandle{x})), own(m.MkApp(q, []ast.ASTHandle{x}))
			set, err := base.NewRuleSet(f.rules)
			ruleMust(t, err)
			rule, err := f.rules.NewRule(px, []ast.ASTHandle{qx}, nil, "step")
			ruleMust(t, err)
			ruleMust(t, set.Add(rule))
			ruleMust(t, rule.Close())
			config := base.DefaultContextSMT2Config()
			config.DeclareVariables = declare
			config.LowLevel = low
			config.Queries = []ast.ASTHandle{px}
			before := m.Stats().LiveReferences
			var out bytes.Buffer
			ruleMust(t, set.WriteContextSMT2(&out, config))
			text := out.String()
			if declare {
				if !strings.Contains(text, "(declare-var A Int)") || strings.Contains(text, "forall") {
					t.Fatal("declared-variable profile", low, text)
				}
			} else if !strings.Contains(text, "forall") || strings.Contains(text, "declare-var") {
				t.Fatal("quantified-rule profile", low, text)
			}
			if !strings.Contains(text, "(query P)") {
				t.Fatal("canonical free-variable query was not direct", text)
			}
			if m.Stats().LiveReferences != before {
				t.Fatal("variable dump leaked owners")
			}
			// An output quantified fact remains quantified, so it requires a fresh
			// zero-arity query relation and an assertion connecting the full formula.
			ruleMust(t, set.SetOutputPredicate(p))
			before = m.Stats().LiveReferences
			config.Queries = nil
			out.Reset()
			ruleMust(t, set.WriteContextSMT2(&out, config))
			bridge := out.String()
			prefix := "(assert (=> (exists"
			if low {
				// mk_smt_pp aliases the quantified antecedent with a let.
				prefix = "(assert (let ("
			}
			for _, part := range []string{"(declare-rel q!", prefix, "(exists", "(Q A)", "(=>", "(query q!"} {
				if !strings.Contains(bridge, part) {
					t.Fatal("quantified output query bridge missing", part, bridge)
				}
			}
			if m.Stats().LiveReferences != before {
				t.Fatal("quantified-query bridge leaked owners")
			}
			ruleMust(t, set.Close())
			for i := range pins {
				ruleMust(t, m.DelRef(&pins[i]))
			}
		}
	}
}

func TestContextSMT2DeclaredVariablesReusePerSortAndKeepDeBruijnOrder(t *testing.T) {
	f := newRuleSetFixture(t)
	m := f.manager
	boolean, err := m.MkBoolSort()
	ruleMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	ruleMust(t, err)
	integer, err := m.GetSort(zero)
	ruleMust(t, err)
	name, err := m.Symbols.Intern("mixed-P")
	ruleMust(t, err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer, integer, boolean}, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, f.rules.RegisterPredicate(p))
	var args []ast.ASTHandle
	for _, spec := range []struct {
		index uint32
		sort  ast.ASTHandle
	}{{2, integer}, {1, integer}, {0, boolean}} {
		v, err := m.MkVar(spec.index, spec.sort)
		ruleMust(t, err)
		args = append(args, v)
	}
	head, err := m.MkApp(p, args)
	ruleMust(t, err)
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	defer func() { ruleMust(t, set.Close()) }()
	for _, label := range []string{"A", "second"} {
		r, err := f.rules.NewRule(head, nil, nil, label)
		ruleMust(t, err)
		ruleMust(t, set.Add(r))
		ruleMust(t, r.Close())
	}
	before := m.Stats().LiveReferences
	var out bytes.Buffer
	ruleMust(t, set.WriteContextSMT2(&out, base.DefaultContextSMT2Config()))
	text := out.String()
	if strings.Count(text, "(declare-var ") != 3 || strings.Count(text, "(mixed-P C B A)") != 2 {
		t.Fatal("per-sort reuse or de Bruijn order", text)
	}
	for _, part := range []string{"(declare-var A Bool)", "(declare-var B Int)", "(declare-var C Int)", ":named A)"} {
		if !strings.Contains(text, part) {
			t.Fatal("declared-variable source profile missing", part, text)
		}
	}
	if m.Stats().LiveReferences != before {
		t.Fatal("mixed-sort dump leaked references")
	}
}
