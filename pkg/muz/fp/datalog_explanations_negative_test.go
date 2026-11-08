package fp

import (
	"errors"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"strconv"
	"testing"
)

func TestDatalogExpressionExplanationNegativeEDBAndMagic(t *testing.T) {
	for _, magic := range []bool{false, true} {
		for _, derived := range []bool{false, true} {
			t.Run("magic="+strconv.FormatBool(magic)+"/derived="+strconv.FormatBool(derived), func(t *testing.T) {
				f := newDirectFactFixture(t)
				e, m := f.engine, f.manager
				must := func(err error) { t.Helper(); fragmentMust(t, err) }
				must(e.SelectEngine("datalog"))
				must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
				must(e.globals.SourceSetName("fixedpoint.datalog.magic_sets_for_queries", strconv.FormatBool(magic)))
				must(e.AddFact(f.declarations["p"], nil))
				if derived {
					rule, err := m.MkImplies(f.applications["p"], f.applications["u"])
					must(err)
					must(e.addRuleEager(rule, "negative-producer"))
				}
				neg, err := m.MkNot(f.applications["u"])
				must(err)
				body, err := m.MkAnd(f.applications["p"], neg)
				must(err)
				rule, err := m.MkImplies(body, f.applications["q"])
				must(err)
				must(e.addRuleEager(rule, "negative-consumer"))
				status, err := e.QueryExpression(f.applications["q"])
				if derived {
					if !errors.Is(err, rel.ErrCompilerUnsupported) {
						t.Fatal("missing negative derived bridge accepted", status, err)
					}
					if _, err = e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
						t.Fatal("unsupported query published answer", err)
					}
					return
				}
				must(err)
				if status != lbool.True {
					t.Fatal("absent negative EDB blocked tuple", status)
				}
				must(e.AddFact(f.declarations["u"], nil))
				status, err = e.QueryExpression(f.applications["q"])
				must(err)
				if status != lbool.False {
					t.Fatal("negative EDB fact failed to block tuple", status)
				}
				answer, err := e.ExpressionAnswer()
				must(err)
				defer func() { must(m.DelRef(&answer)) }()
				got, err := m.RefGet(answer)
				must(err)
				no, err := m.MkFalse()
				must(err)
				if got != no {
					t.Fatal("empty negative query published nonfalse explanation")
				}
			})
		}
	}
}
