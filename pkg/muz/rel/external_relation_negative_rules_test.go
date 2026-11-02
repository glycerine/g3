package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestExecuteExternalBoundNegativeRules(t *testing.T) {
	f := newTableRelationFixture(t)
	rows := make(map[ast.ASTHandle][][]ast.ASTHandle)
	equal := func(a, b []ast.ASTHandle) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	appendUnique := func(target ast.ASTHandle, tuple []ast.ASTHandle) bool {
		for _, old := range rows[target] {
			if equal(old, tuple) {
				return false
			}
		}
		rows[target] = append(rows[target], append([]ast.ASTHandle(nil), tuple...))
		return true
	}
	storeCalls := 0
	duplicateKeys := 0
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return cause
			}
			switch kind {
			case ast.DatalogOpEmpty:
				rows[outputs[0]] = nil
			case ast.DatalogOpStore:
				storeCalls++
				appendUnique(outputs[0], arguments[1:])
			case ast.DatalogOpClone:
				rows[outputs[0]] = append([][]ast.ASTHandle(nil), rows[arguments[0]]...)
			case ast.DatalogOpNegationFilter:
				parameters, parameterErr := f.ast.GetDeclParametersCopy(declaration)
				if parameterErr != nil {
					return parameterErr
				}
				indices := make([]int32, len(parameters))
				for i := range parameters {
					indices[i], parameterErr = f.ast.ParameterInt(parameters[i])
					astTestMust(t, f.ast.DelParameter(&parameters[i]))
					if parameterErr != nil {
						return parameterErr
					}
				}
				if len(indices) == 4 && indices[0] == indices[2] && indices[1] != indices[3] {
					duplicateKeys++
				}
				kept := rows[outputs[0]][:0]
				for _, target := range rows[outputs[0]] {
					blocked := false
					for _, negative := range rows[arguments[1]] {
						matches := true
						for i := 0; i < len(indices); i += 2 {
							matches = matches && target[indices[i]] == negative[indices[i+1]]
						}
						blocked = blocked || matches
					}
					if !blocked {
						kept = append(kept, target)
					}
				}
				rows[outputs[0]] = kept
			case ast.DatalogOpUnion:
				rows[outputs[1]] = nil
				for _, tuple := range rows[arguments[1]] {
					if appendUnique(outputs[0], tuple) {
						appendUnique(outputs[1], tuple)
					}
				}
			default:
				t.Fatalf("unexpected negative rule assignment %d", kind)
			}
			return nil
		},
		Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			if kind != ast.DatalogOpIsEmpty {
				t.Fatalf("unexpected negative rule reduction %d", kind)
			}
			if len(rows[arguments[0]]) == 0 {
				return f.ast.MkTrue()
			}
			return f.ast.MkFalse()
		},
	})
	astTestMust(t, err)
	defer func() { astTestMust(t, plugin.Close()) }()
	rules, err := base.NewRuleManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, rules.Close()) }()
	declare := func(name string, arity int) ast.ASTHandle {
		symbol, cause := f.ast.Symbols.Intern(name)
		astTestMust(t, cause)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = f.boolean
		}
		declaration, cause := f.ast.MkFuncDecl(symbol, domain, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, cause)
		astTestMust(t, rules.RegisterPredicate(declaration))
		return declaration
	}
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		value, cause := f.ast.MkApp(declaration, arguments)
		astTestMust(t, cause)
		return value
	}
	x, err := f.ast.MkVar(0, f.boolean)
	astTestMust(t, err)
	y, err := f.ast.MkVar(1, f.boolean)
	astTestMust(t, err)
	seed := declare("negative-seed", 1)
	lower := declare("negative-lower", 1)
	absent := declare("negative-absent", 1)
	blocked := declare("negative-blocked", 1)
	allowed := declare("negative-allowed", 1)
	nAllow := declare("negative-repeat-allow", 2)
	nBlock := declare("negative-repeat-block", 2)
	repeatAllowed := declare("negative-repeat-allowed-head", 1)
	repeatBlocked := declare("negative-repeat-blocked-head", 1)
	set, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, set.Close()) }()
	add := func(head ast.ASTHandle, tails []ast.ASTHandle, negative []bool, name string) {
		rule, cause := rules.NewRule(head, tails, negative, name)
		astTestMust(t, cause)
		astTestMust(t, set.Add(rule))
		astTestMust(t, rule.Close())
	}
	// The upper-stratum rule is deliberately before its lower-stratum producer.
	add(app(blocked, x), []ast.ASTHandle{app(seed, x), app(lower, x)}, []bool{false, true}, "blocked-before-lower")
	add(app(allowed, x), []ast.ASTHandle{app(seed, x), app(absent, x)}, []bool{false, true}, "allowed")
	add(app(repeatAllowed, x), []ast.ASTHandle{app(seed, x), app(nAllow, x, x)}, []bool{false, true}, "repeat-allowed")
	add(app(repeatBlocked, x), []ast.ASTHandle{app(seed, x), app(nBlock, x, x)}, []bool{false, true}, "repeat-blocked")
	add(app(lower, x), []ast.ASTHandle{app(seed, x)}, nil, "derive-lower")
	add(app(seed, f.truth), nil, nil, "seed-fact")
	add(app(nAllow, f.truth, f.falsehood), nil, nil, "nonmatching-repeat")
	add(app(nBlock, f.truth, f.truth), nil, nil, "matching-repeat")
	astTestMust(t, ExecuteExternalFactRules(plugin, set))
	contains := func(predicate ast.ASTHandle, tuple []ast.ASTHandle) bool {
		relation, cause := plugin.PredicateRelation(predicate)
		astTestMust(t, cause)
		handle, cause := relation.Relation()
		astTestMust(t, cause)
		for _, actual := range rows[handle] {
			if equal(actual, tuple) {
				return true
			}
		}
		return false
	}
	if contains(blocked, []ast.ASTHandle{f.truth}) {
		t.Fatal("upper negative stratum observed an unsaturated lower stratum")
	}
	if !contains(allowed, []ast.ASTHandle{f.truth}) || !contains(repeatAllowed, []ast.ASTHandle{f.truth}) {
		t.Fatal("nonmatching negative tail removed a source tuple")
	}
	if contains(repeatBlocked, []ast.ASTHandle{f.truth}) || duplicateKeys < 2 {
		t.Fatal("repeated negative keys were not mapped to both negative columns", duplicateKeys)
	}

	for _, test := range []struct {
		name     string
		negative ast.ASTHandle
	}{{"constant", app(nBlock, f.truth, x)}, {"unbound", app(absent, y)}} {
		t.Run(test.name, func(t *testing.T) {
			invalid, cause := base.NewRuleSet(rules)
			astTestMust(t, cause)
			defer func() { astTestMust(t, invalid.Close()) }()
			ground := declare("negative-prefix-"+test.name, 1)
			head := declare("negative-invalid-"+test.name, 1)
			fact, cause := rules.NewRule(app(ground, f.truth), nil, nil, "prefix")
			astTestMust(t, cause)
			astTestMust(t, invalid.Add(fact))
			astTestMust(t, fact.Close())
			bad, cause := rules.NewRule(app(head, x), []ast.ASTHandle{app(seed, x), test.negative}, []bool{false, true}, "invalid-negative")
			astTestMust(t, cause)
			astTestMust(t, invalid.Add(bad))
			astTestMust(t, bad.Close())
			before := storeCalls
			if cause = ExecuteExternalFactRules(plugin, invalid); !errors.Is(cause, ErrCompilerUnsupported) || storeCalls != before || contains(ground, []ast.ASTHandle{f.truth}) {
				t.Fatal("invalid negative tail published a fact prefix", cause, before, storeCalls)
			}
		})
	}
	cycle, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, cycle.Close()) }()
	cycleA := declare("negative-cycle-a", 1)
	cycleB := declare("negative-cycle-b", 1)
	for _, spec := range []struct {
		head, negative ast.ASTHandle
		name           string
	}{{cycleA, cycleB, "cycle-a"}, {cycleB, cycleA, "cycle-b"}} {
		rule, cause := rules.NewRule(app(spec.head, x), []ast.ASTHandle{app(seed, x), app(spec.negative, x)}, []bool{false, true}, spec.name)
		astTestMust(t, cause)
		astTestMust(t, cycle.Add(rule))
		astTestMust(t, rule.Close())
	}
	beforeCycle := storeCalls
	if err = ExecuteExternalFactRules(plugin, cycle); !errors.Is(err, ErrCompilerUnsupported) || storeCalls != beforeCycle {
		t.Fatal("negative dependency cycle was not rejected before publication", err, beforeCycle, storeCalls)
	}
}
