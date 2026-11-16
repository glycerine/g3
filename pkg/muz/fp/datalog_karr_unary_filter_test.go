package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestKarrUnaryEmptySourceSkipsFilters(t *testing.T) {
	skipUnreachableInfiniteSortRules(t)
	// Existing concrete empty relation is a nil source register in the compiler.
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "karr_relation"))
	must(g.SourceSetName("fixedpoint.xform.karr", "true"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string, n int) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		must(err)
		sorts := make([]ast.ASTHandle, n)
		for i := range sorts {
			sorts[i] = integer
		}
		d, err := m.MkFuncDecl(s, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := declare("karr-filter-empty-p", 2), declare("karr-filter-empty-q", 1)
	x, err := m.MkVar(0, integer)
	must(err)
	nine, err := m.ArithmeticIntegerNumeralText("9")
	must(err)
	body, err := m.MkApp(p, []ast.ASTHandle{nine, x})
	must(err)
	head, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(rule, ""))
	for i := 0; i < 2; i++ {
		status, err := e.QueryPredicate(q)
		must(err)
		if status != lbool.False {
			t.Fatal("empty relation", status)
		}
	}
}
