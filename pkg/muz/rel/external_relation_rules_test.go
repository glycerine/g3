package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestExecuteExternalFactRulesGroundProjectionAndChain(t *testing.T) {
	f := newTableRelationFixture(t)
	rows := make(map[ast.ASTHandle][][]ast.ASTHandle)
	copyRows := func(source [][]ast.ASTHandle) [][]ast.ASTHandle {
		result := make([][]ast.ASTHandle, len(source))
		for index := range source {
			result[index] = append([]ast.ASTHandle(nil), source[index]...)
		}
		return result
	}
	appendUnique := func(target ast.ASTHandle, values []ast.ASTHandle) {
		for _, existing := range rows[target] {
			if len(existing) != len(values) {
				continue
			}
			equal := true
			for index := range values {
				equal = equal && existing[index] == values[index]
			}
			if equal {
				return
			}
		}
		rows[target] = append(rows[target], append([]ast.ASTHandle(nil), values...))
	}
	stores, renames := 0, 0
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, err := f.ast.GetDeclKind(declaration)
			if err != nil {
				return err
			}
			switch kind {
			case ast.DatalogOpEmpty:
				rows[outputs[0]] = nil
			case ast.DatalogOpStore:
				stores++
				appendUnique(outputs[0], arguments[1:])
			case ast.DatalogOpClone:
				rows[outputs[0]] = copyRows(rows[arguments[0]])
			case ast.DatalogOpUnion:
				for _, values := range rows[arguments[1]] {
					appendUnique(outputs[0], values)
				}
			default:
				t.Fatalf("unexpected assignment operation %d", kind)
			}
			return nil
		},
		Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			kind, err := f.ast.GetDeclKind(declaration)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if kind != ast.DatalogOpProject && kind != ast.DatalogOpRename {
				t.Fatalf("unexpected reduction operation %d", kind)
			}
			sort, err := f.ast.GetRange(declaration)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err := f.ast.MkFreshConst("external-project", sort)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			for _, values := range rows[arguments[0]] {
				if kind == ast.DatalogOpProject {
					appendUnique(result, values[:1])
				} else {
					renames++
					appendUnique(result, []ast.ASTHandle{values[1], values[0]})
				}
			}
			return result, nil
		},
	})
	astTestMust(t, err)
	defer func() { astTestMust(t, plugin.Close()) }()
	rules, err := base.NewRuleManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, rules.Close()) }()
	set, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, set.Close()) }()
	declaration := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		symbol, err := f.ast.Symbols.Intern(name)
		astTestMust(t, err)
		value, err := f.ast.MkFuncDecl(symbol, domain, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, err)
		astTestMust(t, rules.RegisterPredicate(value))
		return value
	}
	p := declaration("external-rule-p", []ast.ASTHandle{f.boolean, f.boolean})
	q := declaration("external-rule-q", []ast.ASTHandle{f.boolean})
	r := declaration("external-rule-r", []ast.ASTHandle{f.boolean})
	s := declaration("external-rule-s", []ast.ASTHandle{f.boolean, f.boolean})
	x, err := f.ast.MkVar(0, f.boolean)
	astTestMust(t, err)
	y, err := f.ast.MkVar(1, f.boolean)
	astTestMust(t, err)
	app := func(decl ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		value, err := f.ast.MkApp(decl, arguments)
		astTestMust(t, err)
		return value
	}
	add := func(head ast.ASTHandle, tail []ast.ASTHandle, name string) {
		rule, err := rules.NewRule(head, tail, nil, name)
		astTestMust(t, err)
		astTestMust(t, set.Add(rule))
		astTestMust(t, rule.Close())
	}
	// Deliberately put dependents before their producers; bounded monotone
	// passes must still propagate the fact through the chain.
	add(app(r, x), []ast.ASTHandle{app(q, x)}, "r-from-q")
	add(app(s, y, x), []ast.ASTHandle{app(p, x, y)}, "s-swaps-p")
	add(app(q, x), []ast.ASTHandle{app(p, x, y)}, "q-from-p")
	add(app(p, f.truth, f.falsehood), nil, "p-fact")
	astTestMust(t, ExecuteExternalFactRules(plugin, set))
	if stores != 1 {
		t.Fatal("ground fact store count", stores)
	}
	for _, tc := range []struct {
		declaration ast.ASTHandle
		want        []ast.ASTHandle
	}{{p, []ast.ASTHandle{f.truth, f.falsehood}}, {q, []ast.ASTHandle{f.truth}}, {r, []ast.ASTHandle{f.truth}}, {s, []ast.ASTHandle{f.falsehood, f.truth}}} {
		relation, err := plugin.PredicateRelation(tc.declaration)
		astTestMust(t, err)
		handle, err := relation.Relation()
		astTestMust(t, err)
		if len(rows[handle]) != 1 || len(rows[handle][0]) != len(tc.want) {
			t.Fatal("external rule result shape", rows[handle], tc.want)
		}
		for index := range tc.want {
			if rows[handle][0][index] != tc.want[index] {
				t.Fatal("external rule result", rows[handle], tc.want)
			}
		}
	}
	if renames == 0 {
		t.Fatal("reordered head did not execute external rename")
	}

	unsupported, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, unsupported.Close()) }()
	rule, err := rules.NewRule(app(r, x), []ast.ASTHandle{app(p, x, y), app(q, x)}, nil, "two-tail")
	astTestMust(t, err)
	astTestMust(t, unsupported.Add(rule))
	astTestMust(t, rule.Close())
	if err = ExecuteExternalFactRules(plugin, unsupported); !errors.Is(err, ErrCompilerUnsupported) {
		t.Fatal("unsupported external rule shape", err)
	}
	if stores != 1 {
		t.Fatal("validation failure partially executed rules", stores)
	}
	repeated, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, repeated.Close()) }()
	rule, err = rules.NewRule(app(s, x, x), []ast.ASTHandle{app(p, x, y)}, nil, "repeated-head")
	astTestMust(t, err)
	astTestMust(t, repeated.Add(rule))
	astTestMust(t, rule.Close())
	if err = ExecuteExternalFactRules(plugin, repeated); !errors.Is(err, ErrCompilerUnsupported) {
		t.Fatal("repeated external head variable", err)
	}
}
