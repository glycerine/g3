package smtcore

import (
	"fmt"
	"testing"
)

func TestSMTRelevancyBaseMarksSurviveScopedInternalization(t *testing.T) {
	for _, baseBoolean := range []bool{false, true} {
		t.Run(fmt.Sprint(baseBoolean), func(t *testing.T) {
			m, _ := smtModelGeneratorFixture(t)
			_, p, s := smtInternalizerFixture(t, m, nil)
			expr := smtRelevancyBool(t, m, "base-mark-scoped-owner")
			root, err := m.NewRef(expr)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, m.DelRef(&root)) })
			if baseBoolean {
				astTestMust(t, s.Internalize(expr, true))
			}
			astTestMust(t, p.MarkRelevant(expr))
			astTestMust(t, p.Propagate())
			limit := len(p.relevant)
			astTestMust(t, s.PushScope())
			astTestMust(t, s.Internalize(expr, false))
			if s.context.enodes.get(expr) == nil {
				t.Fatal("scoped enode not created")
			}
			if !p.IsRelevant(expr) {
				t.Fatal("native owner creation reset an earlier expression relevancy mark")
			}
			astTestMust(t, s.PopScopes(1))
			if s.context.enodes.get(expr) != nil {
				t.Fatal("scoped enode survived")
			}
			_, exists := s.BoolVar(expr)
			if exists != baseBoolean {
				t.Fatal("Boolean lifetime did not follow creation scope")
			}
			if !p.IsRelevant(expr) || len(p.relevant) != limit || p.relevant[limit-1] != expr {
				t.Fatal("native owner retirement erased an earlier expression relevancy mark")
			}
			astTestMust(t, s.Internalize(expr, false))
			if !p.IsRelevant(expr) {
				t.Fatal("recreated owner lost base relevance")
			}
		})
	}
}
