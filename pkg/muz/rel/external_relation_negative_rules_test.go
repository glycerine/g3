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
	arities := make(map[ast.ASTHandle]int)
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
	nullaryComplements, unaryComplements := 0, 0
	conjunctionFilters := 0
	failComplement := false
	failEmpty := false
	failStore := false
	complementFailure := errors.New("negative column total complement refused")
	emptyFailure := errors.New("negative column total empty refused")
	storeFailure := errors.New("constant singleton store refused")
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return cause
			}
			switch kind {
			case ast.DatalogOpEmpty:
				if failEmpty {
					return emptyFailure
				}
				rows[outputs[0]] = nil
				rangeSort, rangeErr := f.ast.GetRange(declaration)
				astTestMust(t, rangeErr)
				columns, columnsErr := f.ast.DatalogRelationColumns(rangeSort)
				astTestMust(t, columnsErr)
				arities[outputs[0]] = len(columns)
			case ast.DatalogOpStore:
				if failStore {
					return storeFailure
				}
				storeCalls++
				appendUnique(outputs[0], arguments[1:])
			case ast.DatalogOpClone:
				rows[outputs[0]] = append([][]ast.ASTHandle(nil), rows[arguments[0]]...)
				arities[outputs[0]] = arities[arguments[0]]
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
			case ast.DatalogOpFilter:
				parameters, parameterErr := f.ast.GetDeclParametersCopy(declaration)
				astTestMust(t, parameterErr)
				condition, conditionErr := f.ast.ParameterAST(parameters[0])
				astTestMust(t, f.ast.DelParameter(&parameters[0]))
				astTestMust(t, conditionErr)
				operator, conditionErr := f.ast.GetDeclKind(condition)
				astTestMust(t, conditionErr)
				if operator == ast.ASTOpAnd {
					conjunctionFilters++
				}
				kept := rows[outputs[0]][:0]
				for _, tuple := range rows[outputs[0]] {
					value := func(term ast.ASTHandle) ast.ASTHandle {
						kind, valueErr := f.ast.GetKind(term)
						astTestMust(t, valueErr)
						if kind != ast.ASTVar {
							return term
						}
						column, valueErr := f.ast.GetVarIndex(term)
						astTestMust(t, valueErr)
						return tuple[column]
					}
					var evaluate func(ast.ASTHandle) bool
					evaluate = func(term ast.ASTHandle) bool {
						op, evalErr := f.ast.GetDeclKind(term)
						astTestMust(t, evalErr)
						count, evalErr := f.ast.GetNumArgs(term)
						astTestMust(t, evalErr)
						switch op {
						case ast.ASTOpAnd:
							for i := uint32(0); i < count; i++ {
								argument, argumentErr := f.ast.GetArg(term, i)
								astTestMust(t, argumentErr)
								if !evaluate(argument) {
									return false
								}
							}
							return true
						case ast.ASTOpEq, ast.ASTOpIff:
							left, argumentErr := f.ast.GetArg(term, 0)
							astTestMust(t, argumentErr)
							right, argumentErr := f.ast.GetArg(term, 1)
							astTestMust(t, argumentErr)
							return value(left) == value(right)
						default:
							t.Fatalf("unexpected interpreted operator %d", op)
							return false
						}
					}
					if evaluate(condition) {
						kept = append(kept, tuple)
					}
				}
				rows[outputs[0]] = kept
			case ast.DatalogOpUnion:
				rows[outputs[1]] = nil
				arities[outputs[1]] = arities[outputs[0]]
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
			if kind == ast.DatalogOpIsEmpty {
				if len(rows[arguments[0]]) == 0 {
					return f.ast.MkTrue()
				}
				return f.ast.MkFalse()
			}
			sort, cause := f.ast.GetRange(declaration)
			astTestMust(t, cause)
			result, cause := f.ast.MkFreshConst("negative-rule-result", sort)
			astTestMust(t, cause)
			switch kind {
			case ast.DatalogOpComplement:
				if failComplement {
					return ast.ASTHandle{}, complementFailure
				}
				arities[result] = arities[arguments[0]]
				if arities[result] == 0 {
					nullaryComplements++
				} else if arities[result] == 1 {
					unaryComplements++
				}
				switch arities[result] {
				case 0:
					if len(rows[arguments[0]]) == 0 {
						rows[result] = [][]ast.ASTHandle{{}}
					}
				case 1:
					rows[result] = [][]ast.ASTHandle{{f.falsehood}, {f.truth}}
				default:
					t.Fatalf("unexpected full relation arity %d", arities[result])
				}
			case ast.DatalogOpJoin:
				arities[result] = arities[arguments[0]] + arities[arguments[1]]
				parameters, parameterErr := f.ast.GetDeclParametersCopy(declaration)
				astTestMust(t, parameterErr)
				indices := make([]int32, len(parameters))
				for i := range parameters {
					indices[i], parameterErr = f.ast.ParameterInt(parameters[i])
					astTestMust(t, f.ast.DelParameter(&parameters[i]))
					astTestMust(t, parameterErr)
				}
				for _, left := range rows[arguments[0]] {
					for _, right := range rows[arguments[1]] {
						matches := true
						for i := 0; i < len(indices); i += 2 {
							matches = matches && left[indices[i]] == right[indices[i+1]]
						}
						if matches {
							appendUnique(result, append(append([]ast.ASTHandle(nil), left...), right...))
						}
					}
				}
			case ast.DatalogOpProject:
				parameters, parameterErr := f.ast.GetDeclParametersCopy(declaration)
				astTestMust(t, parameterErr)
				removed := make(map[int]bool, len(parameters))
				for i := range parameters {
					column, columnErr := f.ast.ParameterInt(parameters[i])
					astTestMust(t, f.ast.DelParameter(&parameters[i]))
					astTestMust(t, columnErr)
					removed[int(column)] = true
				}
				arities[result] = arities[arguments[0]] - len(removed)
				for _, tuple := range rows[arguments[0]] {
					projected := make([]ast.ASTHandle, 0, arities[result])
					for i, value := range tuple {
						if !removed[i] {
							projected = append(projected, value)
						}
					}
					appendUnique(result, projected)
				}
			default:
				t.Fatalf("unexpected negative rule reduction %d", kind)
			}
			return result, nil
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
		astTestPin(t, f.ast, declaration)
		astTestMust(t, rules.RegisterPredicate(declaration))
		return declaration
	}
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		value, cause := f.ast.MkApp(declaration, arguments)
		astTestMust(t, cause)
		return value
	}
	x, err := f.ast.MkVar(0, f.boolean)
	astTestMust(t, err)
	astTestPin(t, f.ast, x)
	y, err := f.ast.MkVar(1, f.boolean)
	astTestMust(t, err)
	astTestPin(t, f.ast, y)
	seed := declare("negative-seed", 1)
	lower := declare("negative-lower", 1)
	absent := declare("negative-absent", 1)
	blocked := declare("negative-blocked", 1)
	allowed := declare("negative-allowed", 1)
	nAllow := declare("negative-repeat-allow", 2)
	nBlock := declare("negative-repeat-block", 2)
	repeatAllowed := declare("negative-repeat-allowed-head", 1)
	repeatBlocked := declare("negative-repeat-blocked-head", 1)
	constantAllowed := declare("negative-constant-allowed", 1)
	constantBlocked := declare("negative-constant-blocked", 1)
	constantUnbound := declare("negative-constant-unbound", 1)
	unboundHead := declare("negative-unbound-head", 1)
	negativeOnly := declare("negative-only-head", 1)
	repeatedUnbound := declare("negative-repeated-unbound", 1)
	interpretedUnbound := declare("negative-interpreted-unbound", 1)
	nullaryMissing := declare("negative-nullary-missing", 0)
	nullaryPresent := declare("negative-nullary-present", 0)
	nullaryAllowed := declare("negative-nullary-allowed", 0)
	nullaryBlocked := declare("negative-nullary-blocked", 0)
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
	add(app(constantAllowed, x), []ast.ASTHandle{app(seed, x), app(nBlock, f.falsehood, x)}, []bool{false, true}, "constant-allowed")
	add(app(constantBlocked, x), []ast.ASTHandle{app(seed, x), app(nBlock, f.truth, x)}, []bool{false, true}, "constant-blocked")
	add(app(constantUnbound, y), []ast.ASTHandle{app(nBlock, f.truth, y)}, []bool{true}, "constant-before-unbound")
	add(app(unboundHead, y), []ast.ASTHandle{app(seed, x), app(nBlock, x, y)}, []bool{false, true}, "unbound-column")
	add(app(negativeOnly, x), []ast.ASTHandle{app(absent, x)}, []bool{true}, "negative-only")
	add(app(repeatedUnbound, y), []ast.ASTHandle{app(nBlock, y, y)}, []bool{true}, "negative-repeated-unbound")
	yTrue, err := f.ast.MkEq(y, f.truth)
	astTestMust(t, err)
	add(app(interpretedUnbound, y), []ast.ASTHandle{app(absent, y), yTrue}, []bool{true, false}, "negative-interpreted-unbound")
	add(app(nullaryAllowed), []ast.ASTHandle{app(nullaryMissing)}, []bool{true}, "nullary-allowed")
	add(app(nullaryBlocked), []ast.ASTHandle{app(nullaryPresent)}, []bool{true}, "nullary-blocked")
	add(app(lower, x), []ast.ASTHandle{app(seed, x)}, nil, "derive-lower")
	add(app(seed, f.truth), nil, nil, "seed-fact")
	add(app(nAllow, f.truth, f.falsehood), nil, nil, "nonmatching-repeat")
	add(app(nBlock, f.truth, f.truth), nil, nil, "matching-repeat")
	add(app(nullaryPresent), nil, nil, "nullary-present")
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
	if !contains(constantAllowed, []ast.ASTHandle{f.truth}) || contains(constantBlocked, []ast.ASTHandle{f.truth}) {
		t.Fatal("constant negative columns did not filter exact tuples")
	}
	if !contains(constantUnbound, []ast.ASTHandle{f.falsehood}) || contains(constantUnbound, []ast.ASTHandle{f.truth}) {
		t.Fatal("constant column was not appended after the unbound full column")
	}
	if !contains(unboundHead, []ast.ASTHandle{f.falsehood}) || contains(unboundHead, []ast.ASTHandle{f.truth}) {
		t.Fatal("unbound negative column did not enumerate its full domain")
	}
	if !contains(negativeOnly, []ast.ASTHandle{f.falsehood}) || !contains(negativeOnly, []ast.ASTHandle{f.truth}) {
		t.Fatal("negative-only rule did not begin from a full column")
	}
	if !contains(repeatedUnbound, []ast.ASTHandle{f.falsehood}) || contains(repeatedUnbound, []ast.ASTHandle{f.truth}) {
		t.Fatal("repeated unbound negative variable did not reuse one full column")
	}
	if contains(interpretedUnbound, []ast.ASTHandle{f.falsehood}) || !contains(interpretedUnbound, []ast.ASTHandle{f.truth}) {
		t.Fatal("interpreted filter did not see the unbound negative column")
	}
	if !contains(nullaryAllowed, nil) || contains(nullaryBlocked, nil) {
		t.Fatal("nullary negation did not filter the nullary total seed")
	}
	fullStart, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, fullStart.Close()) }()
	fullStartHead := declare("negative-full-start-head", 1)
	fullStartRule, err := rules.NewRule(app(fullStartHead, y), []ast.ASTHandle{app(absent, y)}, []bool{true}, "full-start")
	astTestMust(t, err)
	astTestMust(t, fullStart.Add(fullStartRule))
	astTestMust(t, fullStartRule.Close())
	beforeNullary, beforeUnary := nullaryComplements, unaryComplements
	astTestMust(t, ExecuteExternalFactRules(plugin, fullStart))
	if nullaryComplements != beforeNullary || unaryComplements != beforeUnary+1 || !contains(fullStartHead, []ast.ASTHandle{f.truth}) {
		t.Fatal("unbound-only rule did not start directly from its unary total", beforeNullary, nullaryComplements, beforeUnary, unaryComplements)
	}
	interpretedAndUnsafe, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, interpretedAndUnsafe.Close()) }()
	interpretedOnly := declare("interpreted-only-full-column", 1)
	unsafeHead := declare("unsafe-head-full-column", 2)
	unsafeFact := declare("unsafe-empty-body-full-column", 2)
	yEqualsTruth, err := f.ast.MkEq(y, f.truth)
	astTestMust(t, err)
	yEqualsSelf, err := f.ast.MkEq(y, y)
	astTestMust(t, err)
	for _, spec := range []struct {
		head  ast.ASTHandle
		tails []ast.ASTHandle
		name  string
	}{{app(interpretedOnly, y), []ast.ASTHandle{yEqualsTruth, yEqualsSelf}, "interpreted-only-conjunction"}, {app(unsafeHead, y, y), []ast.ASTHandle{app(seed, x)}, "repeated-unsafe-head"}, {app(unsafeFact, y, y), nil, "repeated-unsafe-empty-body"}} {
		rule, cause := rules.NewRule(spec.head, spec.tails, nil, spec.name)
		astTestMust(t, cause)
		astTestMust(t, interpretedAndUnsafe.Add(rule))
		astTestMust(t, rule.Close())
	}
	beforeNullary, beforeUnary = nullaryComplements, unaryComplements
	beforeConjunctions := conjunctionFilters
	astTestMust(t, ExecuteExternalFactRules(plugin, interpretedAndUnsafe))
	if nullaryComplements != beforeNullary || unaryComplements != beforeUnary+1 {
		t.Fatal("interpreted or unsafe variable introduced an extra nullary/full setup", beforeNullary, nullaryComplements, beforeUnary, unaryComplements)
	}
	if conjunctionFilters != beforeConjunctions+2 || contains(interpretedOnly, []ast.ASTHandle{f.falsehood}) || !contains(interpretedOnly, []ast.ASTHandle{f.truth}) {
		t.Fatal("interpreted-only variable was not filtered by one conjunction per round", beforeConjunctions, conjunctionFilters)
	}
	for _, tuple := range [][]ast.ASTHandle{{f.falsehood, f.falsehood}, {f.truth, f.truth}} {
		if !contains(unsafeHead, tuple) || !contains(unsafeFact, tuple) {
			t.Fatal("unsafe head variable did not enumerate or reuse its one full column", tuple)
		}
	}
	if contains(unsafeHead, []ast.ASTHandle{f.falsehood, f.truth}) || contains(unsafeHead, []ast.ASTHandle{f.truth, f.falsehood}) {
		t.Fatal("repeated unsafe head variable used independent full columns")
	}
	for _, test := range []struct {
		name        string
		interpreted bool
	}{{"interpreted", true}, {"unsafe-head", false}} {
		preflight, cause := base.NewRuleSet(rules)
		astTestMust(t, cause)
		prefixDeclaration := declare("full-column-prefix-"+test.name, 0)
		headDeclaration := declare("full-column-failure-"+test.name, 1)
		prefixRule, ruleErr := rules.NewRule(app(prefixDeclaration), nil, nil, "prefix")
		astTestMust(t, ruleErr)
		astTestMust(t, preflight.Add(prefixRule))
		astTestMust(t, prefixRule.Close())
		var tail []ast.ASTHandle
		if test.interpreted {
			condition, conditionErr := f.ast.MkEq(y, f.truth)
			astTestMust(t, conditionErr)
			tail = []ast.ASTHandle{condition}
		} else {
			tail = []ast.ASTHandle{app(seed, x)}
		}
		fullRule, ruleErr := rules.NewRule(app(headDeclaration, y), tail, nil, test.name)
		astTestMust(t, ruleErr)
		astTestMust(t, preflight.Add(fullRule))
		astTestMust(t, fullRule.Close())
		before := storeCalls
		failComplement = true
		cause = ExecuteExternalFactRules(plugin, preflight)
		failComplement = false
		if !errors.Is(cause, complementFailure) || storeCalls != before || contains(prefixDeclaration, nil) {
			t.Fatal("full-column constructor failure published a fact prefix", test.name, cause, before, storeCalls)
		}
		astTestMust(t, preflight.Close())
	}
	failing, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, failing.Close()) }()
	prefix := declare("negative-full-preflight-prefix", 0)
	failingHead := declare("negative-full-preflight-head", 1)
	addFailing := func(head ast.ASTHandle, tails []ast.ASTHandle, negative []bool, name string) {
		rule, cause := rules.NewRule(head, tails, negative, name)
		astTestMust(t, cause)
		astTestMust(t, failing.Add(rule))
		astTestMust(t, rule.Close())
	}
	addFailing(app(prefix), nil, nil, "prefix-before-full-failure")
	addFailing(app(failingHead, y), []ast.ASTHandle{app(absent, y)}, []bool{true}, "full-column-failure")
	beforeFailure := storeCalls
	failComplement = true
	if cause := ExecuteExternalFactRules(plugin, failing); !errors.Is(cause, complementFailure) || storeCalls != beforeFailure {
		t.Fatal("full-column constructor failure published a fact prefix", cause, beforeFailure, storeCalls)
	}
	failComplement = false
	failEmpty = true
	if cause := ExecuteExternalFactRules(plugin, failing); !errors.Is(cause, emptyFailure) || storeCalls != beforeFailure {
		t.Fatal("full-column empty failure published a fact prefix", cause, beforeFailure, storeCalls)
	}
	failEmpty = false
	if contains(prefix, nil) {
		t.Fatal("fact prefix survived failed full-column preflight")
	}
	nullaryFailing, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, nullaryFailing.Close()) }()
	nullaryPrefix := declare("negative-nullary-preflight-prefix", 0)
	nullaryFailureHead := declare("negative-nullary-preflight-head", 0)
	for _, spec := range []struct {
		head     ast.ASTHandle
		tails    []ast.ASTHandle
		negative []bool
		name     string
	}{{app(nullaryPrefix), nil, nil, "nullary-prefix"}, {app(nullaryFailureHead), []ast.ASTHandle{app(nullaryMissing)}, []bool{true}, "nullary-failure"}} {
		rule, cause := rules.NewRule(spec.head, spec.tails, spec.negative, spec.name)
		astTestMust(t, cause)
		astTestMust(t, nullaryFailing.Add(rule))
		astTestMust(t, rule.Close())
	}
	beforeNullaryFailure := storeCalls
	failComplement = true
	if cause := ExecuteExternalFactRules(plugin, nullaryFailing); !errors.Is(cause, complementFailure) || storeCalls != beforeNullaryFailure {
		t.Fatal("nullary complement failure published a fact prefix", cause, beforeNullaryFailure, storeCalls)
	}
	failComplement = false
	failEmpty = true
	if cause := ExecuteExternalFactRules(plugin, nullaryFailing); !errors.Is(cause, emptyFailure) || storeCalls != beforeNullaryFailure {
		t.Fatal("nullary empty failure published a fact prefix", cause, beforeNullaryFailure, storeCalls)
	}
	failEmpty = false
	if contains(nullaryPrefix, nil) {
		t.Fatal("fact prefix survived failed nullary seed preflight")
	}
	constantFailing, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, constantFailing.Close()) }()
	constantPrefix := declare("constant-singleton-preflight-prefix", 0)
	constantFailureHead := declare("constant-singleton-preflight-head", 1)
	for _, spec := range []struct {
		head, tail ast.ASTHandle
		name       string
	}{{app(constantPrefix), ast.ASTHandle{}, "constant-prefix"}, {app(constantFailureHead, f.truth), app(seed, x), "constant-store-failure"}} {
		var tails []ast.ASTHandle
		if !spec.tail.IsNull() {
			tails = []ast.ASTHandle{spec.tail}
		}
		rule, cause := rules.NewRule(spec.head, tails, nil, spec.name)
		astTestMust(t, cause)
		astTestMust(t, constantFailing.Add(rule))
		astTestMust(t, rule.Close())
	}
	beforeConstantFailure := storeCalls
	failStore = true
	if cause := ExecuteExternalFactRules(plugin, constantFailing); !errors.Is(cause, storeFailure) || storeCalls != beforeConstantFailure {
		t.Fatal("constant singleton failure published a fact prefix", cause, beforeConstantFailure, storeCalls)
	}
	failStore = false
	if contains(constantPrefix, nil) {
		t.Fatal("fact prefix survived failed constant singleton preflight")
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
