package spacer

// Source: spacer_context.cpp init_rules creates tail transformers and
// solve_core returns false early only when the query transformer is absent.
import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSpacerTailOnlyQueryHonorsBoundedSearch(t *testing.T) {
	for _, bound := range []uint32{0, 1, 2} {
		t.Run(string(rune('0'+bound)), func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			c, m := f.context, f.context.manager
			depthMust(t, f.manager.RegisterPredicate(f.other))
			head, err := m.MkApp(f.query, nil)
			depthMust(t, err)
			tail, err := m.MkApp(f.other, nil)
			depthMust(t, err)
			rule, err := f.manager.NewRule(head, []ast.ASTHandle{tail}, nil, "tail-only-query")
			depthMust(t, err)
			defer func() { depthMust(t, rule.Close()) }()
			set, err := base.NewRuleSet(f.manager)
			depthMust(t, err)
			defer func() { depthMust(t, set.Close()) }()
			depthMust(t, set.Add(rule))
			depthMust(t, c.UpdateRules(set))
			depthMust(t, c.SetQuery(f.other))
			if c.transformers[f.other] == nil {
				t.Fatal("tail-only predicate transformer absent")
			}
			got, err := c.Solve(0, bound)
			depthMust(t, err)
			want := lbool.Undef
			if bound == 2 {
				want = lbool.False
			}
			if got != want {
				t.Fatalf("tail-only query bound%d: got %s want %s", bound, got, want)
			}
			bounded, err := c.Bounded()
			depthMust(t, err)
			if bounded != (want == lbool.Undef) {
				t.Fatal("wrong bounded status", bounded, want)
			}
		})
	}
}
