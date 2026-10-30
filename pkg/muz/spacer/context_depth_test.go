package spacer

// Source: src/muz/spacer/spacer_context.cpp context::get_cex_depth.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func counterexampleDepthStatistic(t *testing.T, c *Context) uint32 {
	t.Helper()
	s := statistics.New()
	depthMust(t, c.CollectStatistics(s))
	for i := 0; i < s.Size(); i++ {
		if s.IsUint(i) && s.Key(i) == "SPACER cex depth" {
			return s.UintValue(i)
		}
	}
	return 0
}

func TestSpacerCounterexampleDepth(t *testing.T) {
	for _, tc := range []struct {
		name     string
		children [][]int
		depth    uint32
	}{
		{"direct-query-fact", [][]int{nil}, 0},
		{"single-leaf", [][]int{{1}, nil}, 1},
		{"chain", [][]int{{1}, {2}, {3}, nil}, 3},
		{"uneven-branches", [][]int{{1}, {2, 3}, nil, {4}, {5}, nil}, 4},
		{"multiple-query-premises", [][]int{{1, 2}, nil, {3}, nil}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			c, m := f.context, f.context.manager
			set, err := base.NewRuleSet(f.manager)
			depthMust(t, err)
			defer func() { depthMust(t, set.Close()) }()
			var pins []ast.ASTRef
			defer func() { depthMust(t, releaseNormalizeRefs(m, pins)) }()
			apps := make([]ast.ASTHandle, len(tc.children))
			boolean, err := m.MkBoolSort()
			depthMust(t, err)
			for i := range apps {
				decl := f.query
				if i > 0 {
					name, err := m.Symbols.Intern(fmt.Sprintf("depth-node-%d", i))
					depthMust(t, err)
					decl, err = m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
					depthMust(t, err)
					depthMust(t, f.manager.RegisterPredicate(decl))
				}
				app, err := m.MkApp(decl, nil)
				depthMust(t, err)
				pin, err := m.NewRef(app)
				depthMust(t, err)
				pins = append(pins, pin)
				apps[i] = app
			}
			for i, children := range tc.children {
				tail := make([]ast.ASTHandle, len(children))
				for j, child := range children {
					tail[j] = apps[child]
				}
				rule, err := f.manager.NewRule(apps[i], tail, nil, fmt.Sprintf("depth-rule-%d", i))
				depthMust(t, err)
				depthMust(t, set.Add(rule))
				depthMust(t, rule.Close())
			}
			depthMust(t, c.UpdateRules(set))
			if got := counterexampleDepthStatistic(t, c); got != 0 {
				t.Fatal("unsolved depth", got)
			}
			status, err := c.Solve(0, 8)
			depthMust(t, err)
			if status != lbool.True {
				t.Fatal("expected reachable query", status)
			}
			if got := counterexampleDepthStatistic(t, c); got != tc.depth {
				t.Fatalf("counterexample depth = %d, want %d", got, tc.depth)
			}
			trace, err := c.RulesAlongTrace()
			depthMust(t, err)
			defer func() {
				for _, rule := range trace {
					depthMust(t, rule.Close())
				}
			}()
			expected := []int{0}
			if len(tc.children[0]) == 1 {
				expected = append(expected, tc.children[0]...)
				for i := 1; i < len(expected); i++ {
					expected = append(expected, tc.children[expected[i]]...)
				}
			}
			if len(trace) != len(expected) {
				t.Fatalf("trace length = %d, want %d", len(trace), len(expected))
			}
			for i, rule := range trace {
				name, err := rule.Name()
				depthMust(t, err)
				if name != fmt.Sprintf("depth-rule-%d", expected[i]) {
					t.Fatalf("trace rule %d = %s", i, name)
				}
			}
			// Source reset() clears search state, but only reset_statistics clears counters.
			depthMust(t, c.Reset())
			if got := counterexampleDepthStatistic(t, c); got != tc.depth {
				t.Fatal("reset lost statistic", got)
			}
			depthMust(t, c.ResetStatistics())
			if got := counterexampleDepthStatistic(t, c); got != 0 {
				t.Fatal("statistics reset left depth", got)
			}
		})
	}
}

func depthMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
