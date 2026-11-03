package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestUDocPositiveRulesFactsJoinsProjectionAndFixpoint(t *testing.T) {
	f := newDirectFactFixture(t)
	m, engine := f.manager, f.engine
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	name, err := m.Symbols.Intern("udoc-rule-domain")
	must(err)
	domain, err := m.MkDatalogFiniteSort(name, 4)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(label string, arity int) ast.ASTHandle {
		t.Helper()
		symbol, cause := m.Symbols.Intern(label)
		must(cause)
		sorts := make([]ast.ASTHandle, arity)
		for i := range sorts {
			sorts[i] = domain
		}
		declaration, cause := m.MkFuncDecl(symbol, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(engine.RegisterPredicate(declaration))
		return declaration
	}
	edge := declare("udoc-edge", 2)
	reach := declare("udoc-reach", 2)
	repeated := declare("udoc-repeated", 1)
	permuted := declare("udoc-permuted", 3)
	constant := declare("udoc-constant", 2)
	ground := declare("udoc-ground", 1)
	declarations := []ast.ASTHandle{edge, reach, repeated, permuted, constant, ground}
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		result, cause := m.MkApp(declaration, arguments)
		must(cause)
		return result
	}
	values := make([]ast.ASTHandle, 4)
	for i := range values {
		values[i], err = m.MkDatalogNumeral(uint64(i), domain)
		must(err)
	}
	x, err := m.MkVar(0, domain)
	must(err)
	y, err := m.MkVar(1, domain)
	must(err)
	z, err := m.MkVar(2, domain)
	must(err)
	addRule := func(head ast.ASTHandle, tails []ast.ASTHandle, label string) {
		t.Helper()
		rule, cause := engine.rules.NewRule(head, tails, nil, label)
		must(cause)
		must(engine.set.Add(rule))
		must(rule.Close())
	}
	// Deliberately place the recursive consumer before its base producer.
	addRule(app(reach, x, z), []ast.ASTHandle{app(reach, x, y), app(edge, y, z)}, "transitive")
	addRule(app(reach, x, y), []ast.ASTHandle{app(edge, x, y)}, "base")
	addRule(app(repeated, x), []ast.ASTHandle{app(edge, x, x)}, "repeated-tail")
	addRule(app(permuted, y, x, x), []ast.ASTHandle{app(edge, x, y)}, "projection-rename-repeat")
	addRule(app(constant, x, values[3]), []ast.ASTHandle{app(repeated, x)}, "head-constant")
	addRule(app(ground, values[2]), nil, "ground-fact")

	context, err := newDatalogUDocContext(m)
	must(err)
	defer func() { must(context.close()) }()
	for _, declaration := range declarations {
		arity, cause := m.GetArity(declaration)
		must(cause)
		sorts := make([]ast.ASTHandle, arity)
		for i := range sorts {
			sorts[i], cause = m.GetDomain(declaration, uint32(i))
			must(cause)
		}
		relation, cause := context.factory.NewEmpty(sorts)
		must(cause)
		context.seeds[declaration] = relation
		context.order = append(context.order, declaration)
	}
	for _, fact := range [][]ast.ASTHandle{{values[0], values[1]}, {values[1], values[2]}, {values[2], values[2]}} {
		inserted, cause := context.seeds[edge].AddFact(fact)
		must(cause)
		if !inserted {
			t.Fatal("seed fact was not new", fact)
		}
	}
	must(context.snapshot())
	must(context.saturate(engine.set))
	check := func(declaration ast.ASTHandle, fact ...ast.ASTHandle) {
		t.Helper()
		found, cause := context.result[declaration].ContainsFact(fact)
		must(cause)
		if !found {
			t.Fatal("derived UDoc fact missing", declaration, fact)
		}
	}
	check(reach, values[0], values[2])
	check(repeated, values[2])
	check(permuted, values[1], values[0], values[0])
	check(constant, values[2], values[3])
	check(ground, values[2])
}
