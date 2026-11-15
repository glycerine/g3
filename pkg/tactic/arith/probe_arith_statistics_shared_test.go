package arith

// Source: src/ast/for_each_expr.h:27-105, MarkAll goal traversal.
import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestArithmeticStatisticProbeDeduplicatesSharedNumeralAcrossRoots(t *testing.T) {
	_, m := fixDLFixture(t, false)
	x := fixDLConst(t, m, false, "shared-bw-x")
	y := fixDLConst(t, m, false, "shared-bw-y")
	n := fixDLNumber(t, m, false, "3/4")
	g := statisticGoal(t, m, statisticEquality(t, m, x, n), statisticEquality(t, m, y, n))
	p, e := NewArithmeticAverageBitWidthProbe(m.Budget())
	equationMust(t, e)
	defer p.Close()
	got, e := p.Eval(g)
	equationMust(t, e)
	if got != 5 {
		t.Fatal("shared numeral visited repeatedly", got)
	}
}

// AUDIT.md tactic_arith_03-F6: for_each_expr_at descends into quantifier
// bodies, so the degree and bit-width probes see the arithmetic inside them.
func TestArithmeticStatisticsDescendIntoQuantifiers(t *testing.T) {
	_, m := fixDLFixture(t, false)
	x := fixDLConst(t, m, false, "quantified-stat-x")
	sort, e := m.GetSort(x)
	equationMust(t, e)
	v, e := m.MkVar(0, sort)
	equationMust(t, e)
	fixDLPin(t, m, v)
	body := fixDLApp(t, m, 2, fixDLApp(t, m, 9, v, v, v), fixDLNumber(t, m, false, "123456789"))
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
	equationMust(t, e)
	fixDLPin(t, m, q)
	g := statisticGoal(t, m, fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "1")), q)
	degree, e := NewArithmeticMaximumDegreeProbe(m.Budget())
	equationMust(t, e)
	defer degree.Close()
	if got, e := degree.Eval(g); e != nil || got != 3 {
		t.Fatal("arith-max-deg", got, e)
	}
	bw, e := NewArithmeticMaximumBitWidthProbe(m.Budget())
	equationMust(t, e)
	defer bw.Close()
	if got, e := bw.Eval(g); e != nil || got != 27 {
		t.Fatal("arith-max-bw", got, e)
	}
}
