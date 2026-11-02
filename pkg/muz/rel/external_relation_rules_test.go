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
	appendUnique := func(target ast.ASTHandle, values []ast.ASTHandle) bool {
		for _, existing := range rows[target] {
			if len(existing) != len(values) {
				continue
			}
			equal := true
			for index := range values {
				equal = equal && existing[index] == values[index]
			}
			if equal {
				return false
			}
		}
		rows[target] = append(rows[target], append([]ast.ASTHandle(nil), values...))
		return true
	}
	stores, renames, joins, filters, assembledDeltaRows := 0, 0, 0, 0, 0
	emptyCalls, failEmptyAt := 0, 0
	emptyFailure := errors.New("external empty construction refused")
	constructed := make([]ast.ASTHandle, 0)
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, err := f.ast.GetDeclKind(declaration)
			if err != nil {
				return err
			}
			switch kind {
			case ast.DatalogOpEmpty:
				emptyCalls++
				if failEmptyAt != 0 && emptyCalls == failEmptyAt {
					return emptyFailure
				}
				rows[outputs[0]] = nil
				constructed = append(constructed, outputs[0])
			case ast.DatalogOpStore:
				stores++
				appendUnique(outputs[0], arguments[1:])
			case ast.DatalogOpClone:
				rows[outputs[0]] = copyRows(rows[arguments[0]])
			case ast.DatalogOpFilter:
				filters++
				parameters, err := f.ast.GetDeclParametersCopy(declaration)
				if err != nil || len(parameters) != 1 {
					return err
				}
				condition, err := f.ast.ParameterAST(parameters[0])
				astTestMust(t, f.ast.DelParameter(&parameters[0]))
				if err != nil {
					return err
				}
				left, err := f.ast.GetArg(condition, 0)
				if err != nil {
					return err
				}
				right, err := f.ast.GetArg(condition, 1)
				if err != nil {
					return err
				}
				filtered := rows[outputs[0]][:0]
				for _, tuple := range rows[outputs[0]] {
					value := func(term ast.ASTHandle) ast.ASTHandle {
						kind, cause := f.ast.GetKind(term)
						astTestMust(t, cause)
						if kind != ast.ASTVar {
							return term
						}
						column, cause := f.ast.GetVarIndex(term)
						astTestMust(t, cause)
						return tuple[column]
					}
					if value(left) == value(right) {
						filtered = append(filtered, tuple)
					}
				}
				rows[outputs[0]] = filtered
			case ast.DatalogOpUnion:
				if len(outputs) == 2 {
					rows[outputs[1]] = nil
				}
				for _, values := range rows[arguments[1]] {
					if appendUnique(outputs[0], values) && len(outputs) == 2 {
						appendUnique(outputs[1], values)
						if len(values) == 4 {
							assembledDeltaRows++
						}
					}
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
			if kind == ast.DatalogOpIsEmpty {
				if len(rows[arguments[0]]) == 0 {
					return f.ast.MkTrue()
				}
				return f.ast.MkFalse()
			}
			if kind != ast.DatalogOpProject && kind != ast.DatalogOpRename && kind != ast.DatalogOpJoin {
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
			if kind == ast.DatalogOpJoin {
				joins++
				parameters, err := f.ast.GetDeclParametersCopy(declaration)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				defer func() {
					for i := range parameters {
						astTestMust(t, f.ast.DelParameter(&parameters[i]))
					}
				}()
				for _, a := range rows[arguments[0]] {
					for _, b := range rows[arguments[1]] {
						matches := true
						for index := 0; index < len(parameters); index += 2 {
							left, cause := f.ast.ParameterInt(parameters[index])
							if cause != nil {
								return ast.ASTHandle{}, cause
							}
							right, cause := f.ast.ParameterInt(parameters[index+1])
							if cause != nil {
								return ast.ASTHandle{}, cause
							}
							matches = matches && a[left] == b[right]
						}
						if matches {
							appendUnique(result, append(append([]ast.ASTHandle(nil), a...), b...))
						}
					}
				}
				return result, nil
			}
			for _, values := range rows[arguments[0]] {
				if kind == ast.DatalogOpProject {
					parameters, err := f.ast.GetDeclParametersCopy(declaration)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					removed := make(map[int]bool, len(parameters))
					for index := range parameters {
						column, cause := f.ast.ParameterInt(parameters[index])
						astTestMust(t, cause)
						removed[int(column)] = true
						astTestMust(t, f.ast.DelParameter(&parameters[index]))
					}
					projected := make([]ast.ASTHandle, 0, len(values)-len(removed))
					for index, value := range values {
						if !removed[index] {
							projected = append(projected, value)
						}
					}
					appendUnique(result, projected)
				} else {
					renames++
					parameters, err := f.ast.GetDeclParametersCopy(declaration)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					cycle := make([]int32, len(parameters))
					for index := range parameters {
						cycle[index], err = f.ast.ParameterInt(parameters[index])
						astTestMust(t, f.ast.DelParameter(&parameters[index]))
						if err != nil {
							return ast.ASTHandle{}, err
						}
					}
					renamed := append([]ast.ASTHandle(nil), values...)
					for index, destination := range cycle {
						renamed[destination] = values[cycle[(index+1)%len(cycle)]]
					}
					appendUnique(result, renamed)
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
	tail := declaration("external-rule-t", []ast.ASTHandle{f.boolean, f.boolean})
	joined := declaration("external-rule-u", []ast.ASTHandle{f.boolean, f.boolean})
	v := declaration("external-rule-v", []ast.ASTHandle{f.boolean, f.boolean, f.boolean})
	w := declaration("external-rule-w", []ast.ASTHandle{f.boolean})
	h := declaration("external-rule-head-assembly", []ast.ASTHandle{f.boolean, f.boolean, f.boolean, f.boolean})
	x, err := f.ast.MkVar(0, f.boolean)
	astTestMust(t, err)
	y, err := f.ast.MkVar(1, f.boolean)
	astTestMust(t, err)
	z, err := f.ast.MkVar(2, f.boolean)
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
	add(app(joined, z, x), []ast.ASTHandle{app(p, x, y), app(tail, y, z)}, "u-joins-p-t")
	add(app(s, y, x), []ast.ASTHandle{app(p, x, y)}, "s-swaps-p")
	add(app(h, y, y, f.truth, x), []ast.ASTHandle{app(p, x, y)}, "h-repeats-and-extends-p")
	add(app(q, x), []ast.ASTHandle{app(p, x, y)}, "q-from-p")
	add(app(w, x), []ast.ASTHandle{app(v, x, x, f.truth)}, "w-filters-v")
	add(app(v, f.truth, f.truth, f.truth), nil, "v-match")
	add(app(v, f.truth, f.falsehood, f.truth), nil, "v-repeat-miss")
	add(app(tail, f.falsehood, f.falsehood), nil, "t-fact")
	add(app(p, f.truth, f.falsehood), nil, "p-fact")
	astTestMust(t, ExecuteExternalFactRules(plugin, set))
	wantConstructionOrder := []ast.ASTHandle{r, q, joined, p, tail, s, h, w, v}
	if len(constructed) < len(wantConstructionOrder) {
		t.Fatal("missing predicate relation construction callbacks", constructed)
	}
	for index, predicate := range wantConstructionOrder {
		relation, err := plugin.PredicateRelation(predicate)
		astTestMust(t, err)
		handle, err := relation.Relation()
		astTestMust(t, err)
		if constructed[index] != handle {
			t.Fatal("predicate construction order", index, constructed[index], handle)
		}
	}
	if stores != 5 {
		t.Fatal("ground fact store count", stores)
	}
	for _, tc := range []struct {
		declaration ast.ASTHandle
		want        []ast.ASTHandle
	}{{p, []ast.ASTHandle{f.truth, f.falsehood}}, {q, []ast.ASTHandle{f.truth}}, {r, []ast.ASTHandle{f.truth}}, {s, []ast.ASTHandle{f.falsehood, f.truth}}, {tail, []ast.ASTHandle{f.falsehood, f.falsehood}}, {joined, []ast.ASTHandle{f.falsehood, f.truth}}, {w, []ast.ASTHandle{f.truth}}, {h, []ast.ASTHandle{f.falsehood, f.falsehood, f.truth, f.truth}}} {
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
	if renames == 0 || joins == 0 || filters == 0 {
		t.Fatal("richer external operations", renames, joins, filters)
	}
	if assembledDeltaRows == 0 {
		t.Fatal("assembled head never reached the union delta output")
	}
	beforeRepeatFilters := filters
	astTestMust(t, ExecuteExternalFactRules(plugin, set))
	wRelation, err := plugin.PredicateRelation(w)
	astTestMust(t, err)
	wHandle, err := wRelation.Relation()
	astTestMust(t, err)
	if len(rows[wHandle]) != 1 || len(rows[wHandle][0]) != 1 || rows[wHandle][0][0] != f.truth || filters <= beforeRepeatFilters {
		t.Fatal("repeated filtered saturation", rows[wHandle], beforeRepeatFilters, filters)
	}

	unsupported, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, unsupported.Close()) }()
	rule, err := rules.NewRule(app(r, x), []ast.ASTHandle{app(p, x, y), app(q, x), app(tail, y, z)}, nil, "three-tail")
	astTestMust(t, err)
	astTestMust(t, unsupported.Add(rule))
	astTestMust(t, rule.Close())
	if err = ExecuteExternalFactRules(plugin, unsupported); !errors.Is(err, ErrCompilerUnsupported) {
		t.Fatal("unsupported external rule shape", err)
	}
	if stores != 10 {
		t.Fatal("validation failure partially executed rules", stores)
	}
	constantFailure, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, constantFailure.Close()) }()
	for _, item := range []struct {
		head ast.ASTHandle
		tail []ast.ASTHandle
		name string
	}{
		{app(p, f.falsehood, f.truth), nil, "constant-preflight-ground"},
		{app(h, y, y, f.falsehood, x), []ast.ASTHandle{app(p, x, y)}, "constant-preflight-head"},
	} {
		rule, cause := rules.NewRule(item.head, item.tail, nil, item.name)
		astTestMust(t, cause)
		astTestMust(t, constantFailure.Add(rule))
		astTestMust(t, rule.Close())
	}
	beforeConstantFailureStores := stores
	failEmptyAt = emptyCalls + 1
	if err = ExecuteExternalFactRules(plugin, constantFailure); !errors.Is(err, emptyFailure) {
		t.Fatal("constant singleton preflight failure", err)
	}
	failEmptyAt = 0
	if stores != beforeConstantFailureStores {
		t.Fatal("constant singleton failure published a ground prefix", beforeConstantFailureStores, stores)
	}
	repeated, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, repeated.Close()) }()
	rule, err = rules.NewRule(app(s, x, x), []ast.ASTHandle{app(p, x, y)}, nil, "repeated-head")
	astTestMust(t, err)
	astTestMust(t, repeated.Add(rule))
	astTestMust(t, rule.Close())
	astTestMust(t, ExecuteExternalFactRules(plugin, repeated))
	sRelation, err := plugin.PredicateRelation(s)
	astTestMust(t, err)
	sHandle, err := sRelation.Relation()
	astTestMust(t, err)
	foundRepeated := false
	for _, tuple := range rows[sHandle] {
		foundRepeated = foundRepeated || len(tuple) == 2 && tuple[0] == f.truth && tuple[1] == f.truth
	}
	if !foundRepeated {
		t.Fatal("repeated external head variable tuple missing", rows[sHandle])
	}

	nullary, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, nullary.Close()) }()
	a := declaration("external-nullary-a", nil)
	b := declaration("external-nullary-b", nil)
	c := declaration("external-nullary-c", nil)
	addNullary := func(head ast.ASTHandle, tails []ast.ASTHandle, name string) {
		rule, err := rules.NewRule(app(head), tails, nil, name)
		astTestMust(t, err)
		astTestMust(t, nullary.Add(rule))
		astTestMust(t, rule.Close())
	}
	addNullary(c, []ast.ASTHandle{app(a), app(b)}, "nullary-cartesian")
	addNullary(a, nil, "nullary-a")
	addNullary(b, nil, "nullary-b")
	astTestMust(t, ExecuteExternalFactRules(plugin, nullary))
	cRelation, err := plugin.PredicateRelation(c)
	astTestMust(t, err)
	cHandle, err := cRelation.Relation()
	astTestMust(t, err)
	if len(rows[cHandle]) != 1 || len(rows[cHandle][0]) != 0 {
		t.Fatal("nullary Cartesian join", rows[cHandle])
	}

	bits, err := f.ast.BitVectorSort(3)
	astTestMust(t, err)
	edge := declaration("external-recursive-edge", []ast.ASTHandle{bits, bits})
	reach := declaration("external-recursive-reach", []ast.ASTHandle{bits, bits})
	values := make([]ast.ASTHandle, 8)
	valueRefs := make([]ast.ASTRef, len(values))
	defer func() {
		for index := len(valueRefs) - 1; index >= 0; index-- {
			astTestMust(t, f.ast.DelRef(&valueRefs[index]))
		}
	}()
	for index := range values {
		values[index], err = f.ast.BitVectorNumeralUint64(uint64(index), 3)
		astTestMust(t, err)
		valueRefs[index], err = f.ast.NewRef(values[index])
		astTestMust(t, err)
	}
	edgeRelation, err := plugin.PredicateRelation(edge)
	astTestMust(t, err)
	for index := 0; index+1 < len(values); index++ {
		astTestMust(t, edgeRelation.AddFact([]ast.ASTHandle{values[index], values[index+1]}))
	}
	recursive, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, recursive.Close()) }()
	rx, err := f.ast.MkVar(0, bits)
	astTestMust(t, err)
	ry, err := f.ast.MkVar(1, bits)
	astTestMust(t, err)
	rz, err := f.ast.MkVar(2, bits)
	astTestMust(t, err)
	recursiveRule, err := rules.NewRule(app(reach, rx, rz), []ast.ASTHandle{app(reach, rx, ry), app(edge, ry, rz)}, nil, "recursive-reach")
	astTestMust(t, err)
	astTestMust(t, recursive.Add(recursiveRule))
	astTestMust(t, recursiveRule.Close())
	seed, err := rules.NewRule(app(reach, values[0], values[1]), nil, nil, "recursive-seed")
	astTestMust(t, err)
	astTestMust(t, recursive.Add(seed))
	astTestMust(t, seed.Close())
	astTestMust(t, ExecuteExternalFactRules(plugin, recursive))
	reachRelation, err := plugin.PredicateRelation(reach)
	astTestMust(t, err)
	reachHandle, err := reachRelation.Relation()
	astTestMust(t, err)
	foundLast := false
	for _, tuple := range rows[reachHandle] {
		foundLast = foundLast || len(tuple) == 2 && tuple[0] == values[0] && tuple[1] == values[7]
	}
	if !foundLast {
		t.Fatal("recursive delta saturation stopped before the long-chain endpoint", rows[reachHandle])
	}

	canceled, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, canceled.Close()) }()
	canceledRule, err := rules.NewRule(app(p, f.falsehood, f.truth), nil, nil, "canceled-fact")
	astTestMust(t, err)
	astTestMust(t, canceled.Add(canceledRule))
	astTestMust(t, canceledRule.Close())
	beforeCancelStores := stores
	f.ast.ResourceLimit().Cancel()
	if err = ExecuteExternalFactRules(plugin, canceled); err == nil {
		t.Fatal("canceled external saturation continued")
	}
	f.ast.ResourceLimit().ResetCancel()
	if stores != beforeCancelStores {
		t.Fatal("canceled external saturation published a fact", beforeCancelStores, stores)
	}

	construction, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, construction.Close()) }()
	constructionP := declaration("external-construction-p", nil)
	constructionQ := declaration("external-construction-q", nil)
	for _, item := range []struct {
		declaration ast.ASTHandle
		name        string
	}{{constructionP, "construction-p"}, {constructionQ, "construction-q"}} {
		rule, err := rules.NewRule(app(item.declaration), nil, nil, item.name)
		astTestMust(t, err)
		astTestMust(t, construction.Add(rule))
		astTestMust(t, rule.Close())
	}
	beforeConstructionStores := stores
	failEmptyAt = emptyCalls + 2
	if err = ExecuteExternalFactRules(plugin, construction); !errors.Is(err, emptyFailure) {
		t.Fatal("external preflight constructor failure", err)
	}
	failEmptyAt = 0
	if stores != beforeConstructionStores {
		t.Fatal("constructor failure published a rule prefix", beforeConstructionStores, stores)
	}
}
