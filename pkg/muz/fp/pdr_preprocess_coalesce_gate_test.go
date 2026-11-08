package fp

// Source: pdr_dl_interface.cpp:116-132 gates coalescing on a positive
// unfold count and runs it before the requested unfolding rounds.
import (
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDRCoalesceRequiresUnfoldAndPrecedesIt(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr"} {
		for _, rounds := range []int{0, 1, 2} {
			for _, coalesce := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/rounds=%d/coalesce=%t", backend, rounds, coalesce), func(t *testing.T) {
					m, g := newPDRLifecycleServices(t)
					for _, key := range []string{"inline_eager", "inline_linear", "slice", "subsumption_checker"} {
						fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "false"))
					}
					fragmentMust(t, g.SourceSetName("fixedpoint.xform.unfold_rules", fmt.Sprint(rounds)))
					fragmentMust(t, g.SourceSetName("fixedpoint.xform.coalesce_rules", fmt.Sprint(coalesce)))
					e, err := NewPropositionalEngineWithServices(m, g.Context, g)
					fragmentMust(t, err)
					defer func() { fragmentMust(t, e.Close()) }()
					fragmentMust(t, e.SelectEngine(backend))
					var pins []ast.ASTRef
					defer func() {
						for i := len(pins) - 1; i >= 0; i-- {
							fragmentMust(t, m.DelRef(&pins[i]))
						}
					}()
					own := func(h ast.ASTHandle, err error) ast.ASTHandle {
						fragmentMust(t, err)
						r, err := m.NewRef(h)
						fragmentMust(t, err)
						pins = append(pins, r)
						return h
					}
					boolean := own(m.MkBoolSort())
					zero := own(m.ArithmeticIntegerNumeralText("0"))
					one := own(m.ArithmeticIntegerNumeralText("1"))
					integer, err := m.GetSort(zero)
					fragmentMust(t, err)
					decl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
						s, err := m.Symbols.Intern(name)
						fragmentMust(t, err)
						d := own(m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{}))
						fragmentMust(t, e.RegisterPredicate(d))
						return d
					}
					p := decl("pdr-gate-p", []ast.ASTHandle{integer})
					q := decl("pdr-gate-q", nil)
					p0 := own(m.MkApp(p, []ast.ASTHandle{zero}))
					p1 := own(m.MkApp(p, []ast.ASTHandle{one}))
					qa := own(m.MkApp(q, nil))
					fragmentMust(t, e.addRuleEager(p0, "base-zero"))
					fragmentMust(t, e.addRuleEager(p1, "base-one"))
					fragmentMust(t, e.addRuleEager(own(m.MkImplies(p1, qa)), "goal"))
					status, err := e.QueryPredicate(q)
					fragmentMust(t, err)
					if status != lbool.True {
						t.Fatal("query", status)
					}
					n, err := e.pdrRules.Len()
					fragmentMust(t, err)
					names := make([]string, 0, n)
					for i := 0; i < n; i++ {
						r, err := e.pdrRules.Rule(i)
						fragmentMust(t, err)
						name, err := r.Name()
						fragmentMust(t, err)
						names = append(names, name)
					}
					want := []string{"base-zero", "base-one", "goal"}
					if rounds > 0 {
						want = []string{"base-zero", "base-one", "goal;base-one"}
						if coalesce {
							// The merged rule retains the first base name even for the
							// second fact; unfolding must consume that merged rule.
							want = []string{"base-zero", "goal;base-zero"}
						}
					}
					if !reflect.DeepEqual(names, want) {
						t.Fatalf("installed rule names = %v, want %v", names, want)
					}
				})
			}
		}
	}
}
