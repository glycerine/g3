package fp

// Source: dl_mk_filter_rules.cpp introduces a context-local head before
// dl_compiler.cpp binds columns and schedules dependency-first rule execution.

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestExternalIntervalCompilerUsesIsolatedFilterMetadata(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		t.Run(kinds[0], func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			sorts := []ast.ASTHandle{integer, integer}
			p := declareExternalIntervalPredicate(t, manager, engine, "compiler-filter-P", sorts, kinds)
			q := declareExternalIntervalPredicate(t, manager, engine, "compiler-filter-Q", sorts, kinds)
			h := declareExternalIntervalPredicate(t, manager, engine, "compiler-filter-H", sorts, kinds)
			x, err := manager.MkVar(0, integer)
			fragmentMust(t, err)
			y, err := manager.MkVar(1, integer)
			fragmentMust(t, err)
			pxx, err := manager.MkApp(p, []ast.ASTHandle{x, x})
			fragmentMust(t, err)
			qxy, err := manager.MkApp(q, []ast.ASTHandle{x, y})
			fragmentMust(t, err)
			hxy, err := manager.MkApp(h, []ast.ASTHandle{x, y})
			fragmentMust(t, err)
			// Use the raw source order so F(x) is the left binary input.
			raw, err := engine.rules.NewRule(hxy, []ast.ASTHandle{pxx, qxy}, nil, "isolated-filter-input")
			fragmentMust(t, err)
			fragmentMust(t, engine.set.Add(raw))
			fragmentMust(t, raw.Close())
			originalSet, originalRules := engine.set, engine.rules
			originalOrder := append([]ast.ASTHandle(nil), engine.predicateOrder...)

			compiler := engine.externalIntervalCompiler()
			if compiler.manager != manager || compiler.set != originalSet || len(compiler.predicates) != 3 {
				t.Fatal("compiler snapshot lost its borrowed program or predicate metadata")
			}
			// Metadata edits for a local compilation must not alter the
			// persistent declaration signatures, kinds or query-head flags.
			compiler.predicates[p].sorts[0] = ast.ASTHandle{}
			compiler.predicates[p].kinds[0] = "tr_hashtable"
			compiler.queryAuxiliary[p] = true
			if engine.predicates[p].sorts[0] != integer || !externalIntervalKindsEqual(engine.predicates[p].requestedRepresentations, kinds) || engine.auxiliary[p] {
				t.Fatal("compiler snapshot aliases persistent predicate metadata")
			}
			compiler.predicates[p].sorts[0] = integer
			compiler.predicates[p].kinds[0] = kinds[0]
			delete(compiler.queryAuxiliary, p)

			localRules, err := base.NewRuleManager(manager)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, localRules.Close()) }()
			for _, declaration := range originalOrder {
				fragmentMust(t, localRules.RegisterPredicate(declaration))
			}
			localSet, err := base.NewRuleSet(localRules)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, localSet.Close()) }()
			original, err := originalSet.Rule(0)
			fragmentMust(t, err)
			fragmentMust(t, localSet.Add(original))
			fragmentMust(t, original.Close())
			plugin := transforms.NewFilterRulesPlugin(manager, localRules, 2000)
			defer func() { fragmentMust(t, plugin.Close()) }()
			filtered, err := plugin.Transform(localSet)
			fragmentMust(t, err)
			if filtered == nil {
				t.Fatal("real repeated-tail filter did not transform the source rule")
			}
			defer func() { fragmentMust(t, filtered.Close()) }()
			fresh := plugin.FreshPredicates()
			if len(fresh) != 1 {
				t.Fatal("expected one repeated-column filter head", len(fresh))
			}
			filter := fresh[0]
			origin, found := plugin.FreshPredicateOrigin(filter)
			if !found || origin != p {
				t.Fatal("filter helper lost its source representation origin")
			}
			arity, err := manager.GetArity(filter)
			fragmentMust(t, err)
			if arity != 1 {
				t.Fatal("repeated-column filter did not project its signature", arity)
			}
			domain, err := manager.GetDomain(filter, 0)
			fragmentMust(t, err)
			descriptor := &externalIntervalDatalogPredicate{declaration: filter, sorts: []ast.ASTHandle{domain}, kinds: append([]string(nil), compiler.predicates[origin].kinds...)}
			compiler.predicates[filter] = descriptor

			// Store the consumer first to make dependency scheduling observable.
			compiledSet, err := base.NewRuleSet(localRules)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, compiledSet.Close()) }()
			for _, declaration := range []ast.ASTHandle{h, filter} {
				rules, cause := filtered.PredicateRules(declaration)
				fragmentMust(t, cause)
				if len(rules) != 1 {
					t.Fatal("unexpected transformed source producers", len(rules))
				}
				fragmentMust(t, compiledSet.Add(rules[0]))
				fragmentMust(t, rules[0].Close())
			}
			compiler.set = compiledSet
			resetExternalIntervalUnaryCallbacks(callbacks)
			program, err := compiler.externalIntervalProgramPlan()
			fragmentMust(t, err)
			if program == nil || len(program.rules) != 2 || !reflect.DeepEqual(program.schedule, []int{1, 0}) {
				t.Fatal("compiler did not schedule the real filter before its consumer")
			}
			unary := program.rules[1].identity
			if unary == nil || unary.head != filter || unary.tail != p || unary.transform == nil ||
				!reflect.DeepEqual(unary.transform.identical, [][]uint32{{0, 1}}) || !externalIntervalColumnsEqual(unary.transform.removed, []uint32{0}) || len(unary.transform.renames) != 0 {
				t.Fatal("compiler lost the repeated-column filter and last-occurrence projection")
			}
			join := program.rules[0].cartesian
			if join == nil || join.head != h || join.left != filter || join.right != q ||
				!externalIntervalColumnsEqual(join.leftColumns, []uint32{0}) || !externalIntervalColumnsEqual(join.rightColumns, []uint32{0}) ||
				!externalIntervalColumnsEqual(join.removed, []uint32{0}) || len(join.renames) != 0 {
				t.Fatal("compiler lost the filtered binary join keys or projected columns")
			}
			delete(compiler.predicates, filter)
			if _, err = compiler.externalIntervalProgramPlan(); !errors.Is(err, ErrExternalIntervalDatalogFragment) {
				t.Fatal("compiler accepted a generated declaration without metadata", err)
			}
			compiler.predicates[filter] = descriptor
			compiler.queryAuxiliary[filter] = true
			if _, err = compiler.externalIntervalProgramPlan(); !errors.Is(err, ErrExternalIntervalDatalogFragment) {
				t.Fatal("compiler lost the query-auxiliary boundary", err)
			}
			delete(compiler.queryAuxiliary, filter)
			_, err = compiler.externalIntervalProgramPlan()
			fragmentMust(t, err)
			// The persistent wrapper still sees the original unsupported
			// repeated binary tail; local metadata does not activate a port.
			if _, _, _, err = engine.externalIntervalRulePlans(); !errors.Is(err, ErrExternalIntervalDatalogFragment) {
				t.Fatal("compiler extraction changed the existing engine boundary", err)
			}
			registered, err := engine.rules.IsRegisteredPredicate(filter)
			fragmentMust(t, err)
			count, err := originalSet.Len()
			fragmentMust(t, err)
			if registered || engine.predicates[filter] != nil || engine.auxiliary[filter] || engine.set != originalSet || engine.rules != originalRules || count != 1 || len(engine.predicates) != 3 || !reflect.DeepEqual(engine.predicateOrder, originalOrder) {
				t.Fatal("isolated compilation changed the persistent rule registry")
			}
			if len(callbacks.operations) != 0 || engine.externalIntervalDatalog != nil {
				t.Fatal("compilation invoked external callbacks or activated relations", callbacks.operations)
			}
			fragmentMust(t, compiledSet.Close())
			fragmentMust(t, filtered.Close())
			fragmentMust(t, plugin.Close())
			fragmentMust(t, localSet.Close())
			fragmentMust(t, localRules.Close())
			if _, err = manager.GetKind(filter); !errors.Is(err, ast.ErrASTHandle) {
				t.Fatal("isolated compiler scope left a generated predicate owner", err)
			}
		})
	}
}
