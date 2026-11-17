package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDatalogExpressionExplanationsCarryNamedProofAndSurviveScope(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
	must(e.AddFact(f.declarations["p"], nil))
	step, err := m.MkImplies(f.applications["p"], f.applications["q"])
	must(err)
	must(e.addRuleEager(step, "explained-step"))
	status, err := e.QueryExpression(f.applications["q"])
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	answer, err := e.ExpressionAnswer()
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	root, err := m.RefGet(answer)
	must(err)
	var walk func(ast.ASTHandle)
	names := map[string]bool{}
	walk = func(h ast.ASTHandle) {
		kind, err := m.GetKind(h)
		must(err)
		if kind != ast.ASTApp {
			return
		}
		d, err := m.GetDecl(h)
		must(err)
		sym, err := m.GetASTName(d)
		must(err)
		name, _, err := m.Symbols.BareString(sym)
		must(err)
		names[name] = true
		n, err := m.GetNumArgs(h)
		must(err)
		for i := uint32(0); i < n; i++ {
			arg, err := m.GetArg(h, i)
			must(err)
			walk(arg)
		}
	}
	walk(root)
	if !names["explained-step"] || !names["fact"] {
		t.Fatal("answer lacks source proof constructors", names)
	}
	// The transformed product has already been scoped away; the owning answer
	// must also outlive destruction of the expression-query clone.
	status, err = e.QueryPredicate(f.declarations["q"])
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	held, err := m.RefGet(answer)
	must(err)
	if held != root {
		t.Fatal("answer owner changed after query cleanup")
	}
	if e.datalogExplanationAnswer != (ast.ASTRef{}) {
		t.Fatal("direct relation query enabled expression explanations")
	}
}

func TestDatalogExpressionExplanationOpenQueryKeepsFiniteAndProofColumns(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
	b, err := m.MkBoolSort()
	must(err)
	sym, err := m.Symbols.Intern("open-explanation-P")
	must(err)
	d, err := m.MkFuncDecl(sym, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(d))
	yes, err := m.MkTrue()
	must(err)
	must(e.AddFact(d, []ast.ASTHandle{yes}))
	x, err := m.MkVar(0, b)
	must(err)
	query, err := m.MkApp(d, []ast.ASTHandle{x})
	must(err)
	ref, err := m.NewRef(query)
	must(err)
	defer func() { must(m.DelRef(&ref)) }()
	status, err := e.QueryExpression(query)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	answer, err := e.ExpressionAnswer()
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	root, err := m.RefGet(answer)
	must(err)
	variables := map[uint32]ast.ASTHandle{}
	var visit func(ast.ASTHandle)
	visit = func(h ast.ASTHandle) {
		kind, err := m.GetKind(h)
		must(err)
		if kind == ast.ASTVar {
			index, err := m.GetVarIndex(h)
			must(err)
			sort, err := m.GetSort(h)
			must(err)
			variables[index] = sort
			return
		}
		if kind != ast.ASTApp {
			return
		}
		n, err := m.GetNumArgs(h)
		must(err)
		for i := uint32(0); i < n; i++ {
			arg, err := m.GetArg(h, i)
			must(err)
			visit(arg)
		}
	}
	visit(root)
	proofSort, err := m.MkDatalogRuleSort()
	must(err)
	if variables[0] != b || variables[1] != proofSort || len(variables) != 2 {
		t.Fatal("answer lost finite key or shifted proof column", variables)
	}
}

func TestDatalogRelationExplanationFormulaCheckConstantHeadWithoutPositiveTail(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.explanations_on_relation_level", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "hashtable"))
	boolean, err := m.MkBoolSort()
	must(err)
	bits, err := m.BitVectorSort(2)
	must(err)
	name, err := m.Symbols.Intern("relation-formula-constant-head")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, bits}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(predicate))
	three, err := m.BitVectorNumeralUnsigned(3, 2)
	must(err)
	truth, err := m.MkTrue()
	must(err)
	head, err := m.MkApp(predicate, []ast.ASTHandle{truth, three})
	must(err)
	headRef, err := m.NewRef(head)
	must(err)
	defer func() { must(m.DelRef(&headRef)) }()
	must(e.addRuleEager(head, "constant-head-no-tail"))
	status, err := e.QueryExpression(head)
	must(err)
	if status != lbool.True {
		t.Fatal("constant-head formula-checked explanation", status)
	}
	answer, err := e.ExpressionAnswer()
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	root, err := m.RefGet(answer)
	must(err)
	if !datalogExplanationFormulaNames(t, m, root)["constant-head-no-tail"] {
		t.Fatal("constant-head answer lacks named proof")
	}
}
