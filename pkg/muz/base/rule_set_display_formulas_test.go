package base_test

// Source: dl_context.cpp get_rules_as_formulas output-clause partitioning.

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/memory"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestRuleSetDisplayFormulasPartitionOrderAppendAndOwnership(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q", "R")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	defer func() { ruleMust(t, set.Close()) }()
	for _, r := range []*base.Rule{f.rule("R", nil, nil), f.rule("P", []string{"R"}, nil), f.rule("Q", nil, nil), f.rule("P", nil, nil), f.rule("P", []string{"Q"}, nil)} {
		ruleMust(t, set.Add(r))
		ruleMust(t, r.Close())
	}
	ruleMust(t, set.SetOutputPredicate(f.decl["P"]))
	before := f.manager.Stats().LiveReferences
	seed, err := f.manager.NewRef(f.app["R"])
	ruleMust(t, err)
	rules := []ast.ASTRef{seed}
	var queries []ast.ASTRef
	names := []string{"seed"}
	defer func() {
		for i := range rules {
			ruleMust(t, f.manager.DelRef(&rules[i]))
		}
		for i := range queries {
			ruleMust(t, f.manager.DelRef(&queries[i]))
		}
	}()
	ruleMust(t, set.AppendDisplayFormulas(&rules, &queries, &names))
	if len(rules) != 3 || len(queries) != 3 || len(names) != 3 || names[0] != "seed" || names[1] != "R" || names[2] != "Q" {
		t.Fatal("partition", len(rules), len(queries), names)
	}
	for i, want := range []ast.ASTHandle{f.app["R"], f.app["P"], f.app["Q"]} {
		got, err := f.manager.RefGet(queries[i])
		ruleMust(t, err)
		if got != want {
			t.Fatal("query source order", i)
		}
	}
	for i := range rules {
		ruleMust(t, f.manager.DelRef(&rules[i]))
	}
	rules = nil
	for i := range queries {
		ruleMust(t, f.manager.DelRef(&queries[i]))
	}
	queries = nil
	if f.manager.Stats().LiveReferences != before {
		t.Fatal("display formula ownership leaked")
	}
	// The returned owners independently keep formulas alive after set disposal.
	ruleMust(t, set.AppendDisplayFormulas(&rules, &queries, &names))
	ruleMust(t, set.Close())
	for _, ref := range append(append([]ast.ASTRef(nil), rules...), queries...) {
		h, err := f.manager.RefGet(ref)
		ruleMust(t, err)
		if h.IsNull() {
			t.Fatal("retired display formula")
		}
	}
}

func TestRuleSetDisplayFormulasQuantifiedQueriesAndFacts(t *testing.T) {
	f := newRuleSetFixture(t)
	m := f.manager
	boolean, err := m.MkBoolSort()
	ruleMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	ruleMust(t, err)
	integer, err := m.GetSort(zero)
	ruleMust(t, err)
	var pins []ast.ASTRef
	defer func() {
		for i := range pins {
			ruleMust(t, m.DelRef(&pins[i]))
		}
	}()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		ruleMust(t, err)
		ref, e := m.NewRef(h)
		ruleMust(t, e)
		pins = append(pins, ref)
		return h
	}
	declare := func(name string) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		ruleMust(t, err)
		d := own(m.MkFuncDecl(s, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{}))
		ruleMust(t, f.rules.RegisterPredicate(d))
		return d
	}
	p, q := declare("display-P"), declare("display-Q")
	x := own(m.MkVar(0, integer))
	px, qx := own(m.MkApp(p, []ast.ASTHandle{x})), own(m.MkApp(q, []ast.ASTHandle{x}))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	defer func() { ruleMust(t, set.Close()) }()
	for _, tails := range [][]ast.ASTHandle{{qx}, nil} {
		r, err := f.rules.NewRule(px, tails, nil, "quantified-output")
		ruleMust(t, err)
		ruleMust(t, set.Add(r))
		ruleMust(t, r.Close())
	}
	ruleMust(t, set.SetOutputPredicate(p))
	var rules, queries []ast.ASTRef
	var names []string
	defer func() {
		for i := range rules {
			ruleMust(t, m.DelRef(&rules[i]))
		}
		for i := range queries {
			ruleMust(t, m.DelRef(&queries[i]))
		}
	}()
	ruleMust(t, set.AppendDisplayFormulas(&rules, &queries, &names))
	if len(rules) != 0 || len(names) != 0 || len(queries) != 2 {
		t.Fatal("quantified partition", len(rules), len(queries), names)
	}
	for i, ref := range queries {
		h, err := m.RefGet(ref)
		ruleMust(t, err)
		forall, err := m.IsForall(h)
		ruleMust(t, err)
		if forall != (i == 1) {
			t.Fatal("output clause quantifier polarity", i, forall)
		}
		body, err := m.GetQuantifierBody(h)
		ruleMust(t, err)
		want := qx
		if i == 1 {
			want = px
		}
		if body != want {
			t.Fatal("output query body", i)
		}
		count, err := m.GetNumDeclarations(h)
		ruleMust(t, err)
		if count != 1 {
			t.Fatal("binder count", count)
		}
		sort, err := m.GetDeclarationSort(h, 0)
		ruleMust(t, err)
		if sort != integer {
			t.Fatal("binder sort")
		}
		name, err := m.GetDeclarationName(h, 0)
		ruleMust(t, err)
		nameText, err := m.Symbols.String(name)
		ruleMust(t, err)
		if nameText != "A" {
			t.Fatal("source binder name", nameText)
		}
		weight, err := m.GetQuantifierWeight(h)
		ruleMust(t, err)
		if weight != 0 {
			t.Fatal("source default query weight", weight)
		}
	}
}

func TestRuleSetDisplayFormulasFailurePrefixOwnershipAndRetry(t *testing.T) {
	complete, failures := false, 0
	for allowance := int64(0); allowance < 512 && !complete; allowance++ {
		t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			f := newRuleSetFixture(t, "Q")
			m := f.manager
			boolean, err := m.MkBoolSort()
			ruleMust(t, err)
			name, err := m.Symbols.Intern("display-failure-P")
			ruleMust(t, err)
			p, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			ruleMust(t, err)
			ruleMust(t, f.rules.RegisterPredicate(p))
			x, err := m.MkVar(0, boolean)
			ruleMust(t, err)
			px, err := m.MkApp(p, []ast.ASTHandle{x})
			ruleMust(t, err)
			pxOwner, err := m.NewRef(px)
			ruleMust(t, err)
			defer func() { ruleMust(t, m.DelRef(&pxOwner)) }()
			set, err := base.NewRuleSet(f.rules)
			ruleMust(t, err)
			defer func() { ruleMust(t, set.Close()) }()
			fact := f.rule("Q", nil, nil)
			ruleMust(t, set.Add(fact))
			ruleMust(t, fact.Close())
			for _, tails := range [][]ast.ASTHandle{{f.app["Q"]}, nil} {
				r, err := f.rules.NewRule(px, tails, nil, "output")
				ruleMust(t, err)
				ruleMust(t, set.Add(r))
				ruleMust(t, r.Close())
			}
			ruleMust(t, set.SetOutputPredicate(p))
			before := m.Stats().LiveReferences
			seed, err := m.NewRef(f.app["Q"])
			ruleMust(t, err)
			rules, queries, names := []ast.ASTRef{seed}, []ast.ASTRef(nil), []string{"seed"}
			release := func() {
				for i := range rules {
					ruleMust(t, m.DelRef(&rules[i]))
				}
				for i := range queries {
					ruleMust(t, m.DelRef(&queries[i]))
				}
				rules = nil
				queries = nil
				names = nil
			}
			m.Budget().FailAfter(allowance)
			failure := set.AppendDisplayFormulas(&rules, &queries, &names)
			m.Budget().FailAfter(-1)
			if len(names) != len(rules) {
				t.Fatal("failed conversion split rule/name prefix")
			}
			for _, ref := range append(append([]ast.ASTRef(nil), rules...), queries...) {
				h, err := m.RefGet(ref)
				ruleMust(t, err)
				if h.IsNull() {
					t.Fatal("failed conversion published an empty owner")
				}
			}
			release()
			if failure != nil {
				failures++
				var cleanup memory.ConstructionFailure
				if errors.As(failure, &cleanup) {
					ruleMust(t, cleanup.Cleanup())
				}
				var resource memory.Failure
				if !errors.Is(failure, arena.ErrInjectedFailure) && (!errors.As(failure, &resource) || resource.Kind != memory.FailureMemout) {
					t.Fatal("unexpected formula conversion failure", failure)
				}
			} else {
				complete = true
			}
			if m.Stats().LiveReferences != before {
				t.Fatal("failed conversion lost a temporary owner", allowance, failure)
			}
			ruleMust(t, set.AppendDisplayFormulas(&rules, &queries, &names))
			if len(rules) != 1 || len(queries) != 2 || len(names) != 1 {
				t.Fatal("retry partition", len(rules), len(queries), names)
			}
			release()
			if m.Stats().LiveReferences != before {
				t.Fatal("retry leaked formula owners")
			}
		})
	}
	if !complete || failures == 0 {
		t.Fatal("incomplete display formula allocation campaign", complete, failures)
	}
}
