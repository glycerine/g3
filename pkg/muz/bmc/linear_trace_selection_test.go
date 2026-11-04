package bmc_test

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
)

// linear::get_model scans a predicate's selectors in rule order and chooses
// the first one interpreted as true; absent and false interpretations are skipped.
func TestLinearTraceSelectsFirstTrueRule(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m, err := ctx.Manager()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	sym, err := m.Symbols.Intern("linear-selection")
	must(err)
	pred, err := m.MkFuncDecl(sym, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	head, err := m.MkApp(pred, nil)
	must(err)
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	must(rm.RegisterPredicate(pred))
	set, err := base.NewRuleSet(rm)
	must(err)
	defer func() { must(set.Close()) }()
	for i := 0; i < 3; i++ {
		r, e := rm.NewRule(head, nil, nil, fmt.Sprintf("fact-%d", i))
		must(e)
		must(set.Add(r))
		must(r.Close())
	}
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	truth, err := m.MkTrue()
	must(err)
	falsity, err := m.MkFalse()
	must(err)
	for _, tc := range []struct {
		name   string
		values []int
		want   string
	}{
		{"first true", []int{1, 1, 1}, "fact-0"},
		{"skip false and absent", []int{-1, 0, 1}, "fact-2"},
		{"no true selector", []int{-1, 0, -1}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			md, e := model.NewModelCore(m)
			must(e)
			defer func() { must(md.Close()) }()
			for i, v := range tc.values {
				if v == 0 {
					continue
				}
				name, e := m.Symbols.Intern(fmt.Sprintf("rule:linear-selection#0_%d", i))
				must(e)
				c, e := m.MkConst(name, boolean)
				must(e)
				d, e := m.GetDecl(c)
				must(e)
				value := truth
				if v < 0 {
					value = falsity
				}
				must(md.RegisterConstant(d, value))
			}
			trace, e := compiler.LinearTrace(set, pred, 0, md)
			if tc.want == "" {
				if e == nil || len(trace) != 0 {
					t.Fatalf("expected no selected rule, got %v, %v", trace, e)
				}
				return
			}
			must(e)
			if len(trace) != 1 {
				t.Fatalf("trace length = %d", len(trace))
			}
			name, e := trace[0].Name()
			must(e)
			must(trace[0].Close())
			if name != tc.want {
				t.Fatalf("selected %q, want %q", name, tc.want)
			}
		})
	}
}
