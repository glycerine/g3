package smtcore

// Source: src/smt/theory_array_base.cpp:70,106,131,162,174,184,201,205.

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type smtArrayClosedTraceWriter struct{}

func (smtArrayClosedTraceWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func smtArrayTraceManager(t *testing.T, tags *diagnostic.TraceTags) *ast.ASTManager {
	t.Helper()
	symbols, numerals, budget := astTestOwners(t)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget, TraceTags: tags})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, manager.Close()) })
	return manager
}

func TestSMTArrayStoreReadSourceTracesAndRefusal(t *testing.T) {
	for _, closed := range []bool{false, true} {
		t.Run(map[bool]string{false: "output", true: "closed"}[closed], func(t *testing.T) {
			var output bytes.Buffer
			var writer io.Writer = &output
			if closed {
				writer = smtArrayClosedTraceWriter{}
			}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			for _, tag := range []string{"array", "array_axiom", "array_bug", "array_map_bug", "mk_var_bug"} {
				tags.Enable(tag)
			}
			manager := smtArrayTraceManager(t, tags)
			family, err := manager.RegisterArrayPlugin()
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, manager, nil)
			theory, err := NewSMTArrayTheory(family)
			astTestMust(t, err)
			astTestMust(t, state.RegisterTheory(theory))
			indexSort, indices := smtArrayQueueIndices(t, manager, "array-trace-index-", 3)
			boolean, err := manager.MkBoolSort()
			astTestMust(t, err)
			arraySort, err := manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{indexSort, indexSort}), boolean)
			astTestMust(t, err)
			base, err := manager.MkConst(astTestSymbol(t, manager, "array-trace-base"), arraySort)
			astTestMust(t, err)
			value, err := manager.MkTrue()
			astTestMust(t, err)
			store, err := manager.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, indices[0], indices[1], value})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(store, false))
			read, err := manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{store, indices[0], indices[2]})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(read, true))
			astTestMust(t, state.markRelevant(read))
			astTestMust(t, state.propagateTheoryRelevancy())
			if added, err := theory.queueStoreReadAxiom(read, store, false); err != nil || added {
				t.Fatal("duplicate store/read fingerprint", added, err)
			}
			astTestMust(t, theory.Propagate())
			function, err := manager.MkFuncDecl(astTestSymbol(t, manager, "array-trace-function"), []ast.ASTHandle{indexSort, indexSort}, boolean, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			asArray, err := manager.MkAsArray(function)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(asArray, false))
			if !theory.unsupported {
				t.Fatal("as-array did not retain the unsupported-operation marker")
			}
			constant, err := manager.MkConstArray(arraySort, value)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(constant, false))
			baseVariable := state.context.enodes.get(base).TheoryVar(family)
			constantVariable := state.context.enodes.get(constant).TheoryVar(family)
			astTestMust(t, theory.setArrayUpward(uint32(baseVariable)))
			astTestMust(t, theory.setStoreBaseUpward(state.context.enodes.get(store)))
			astTestMust(t, theory.NewEquality(baseVariable, constantVariable))
			if closed {
				return
			}
			if !tags.ShouldTrace("array") {
				if output.Len() != 0 {
					t.Fatal("compiled-out array trace executed", output.String())
				}
				return
			}
			trace := output.String()
			for _, want := range []string{
				"[array] assert_store_axiom2 src/smt/theory_array_base.cpp:201",
				"[array] assert_store_axiom2 src/smt/theory_array_base.cpp:205",
				"[mk_var_bug] mk_select src/smt/theory_array_base.cpp:45",
				"[array] assert_store_axiom1_core src/smt/theory_array_base.cpp:106",
				"[mk_var_bug] assert_store_axiom1_core src/smt/theory_array_base.cpp:113",
				"[array] assert_store_axiom2_core src/smt/theory_array_base.cpp:131",
				"[array_bug] assert_store_axiom2_core src/smt/theory_array_base.cpp:162",
				"[array] assert_store_axiom2_core src/smt/theory_array_base.cpp:174",
				"[array] assert_store_axiom2_core src/smt/theory_array_base.cpp:184",
				"[array_map_bug] assert_store_axiom2_core src/smt/theory_array_base.cpp:185",
				"[array_axiom] assert_axiom src/smt/theory_array_base.cpp:70",
				"[array] found_unsupported_op src/smt/theory_array_base.cpp:36",
				"[array] internalize_term src/smt/theory_array_full.cpp:260",
				"[array] set_prop_upward src/smt/theory_array_full.cpp:115",
				"[array] set_prop_upward src/smt/theory_array_full.cpp:143",
				"[array] add_parent_select src/smt/theory_array_full.cpp:394",
				"[array] relevant_eh src/smt/theory_array_full.cpp:428",
				"[array] merge_eh src/smt/theory_array_full.cpp:359",
			} {
				if !strings.Contains(trace, want) {
					t.Fatalf("missing source array trace %q in %q", want, trace)
				}
			}
		})
	}
}

func TestSMTArrayExtensionalityAndSharedSourceTraces(t *testing.T) {
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	for _, tag := range []string{"array_ext", "ext", "array_axiom", "array_shared", "array_bug"} {
		tags.Enable(tag)
	}
	manager := smtArrayTraceManager(t, tags)
	family, err := manager.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, manager, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	arraySort, err := manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	astTestMust(t, err)

	a, err := manager.MkConst(astTestSymbol(t, manager, "array-ext-trace-a"), arraySort)
	astTestMust(t, err)
	b, err := manager.MkConst(astTestSymbol(t, manager, "array-ext-trace-b"), arraySort)
	astTestMust(t, err)
	index, err := manager.MkTrue()
	astTestMust(t, err)
	left, err := manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, index})
	astTestMust(t, err)
	right, err := manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{b, index})
	astTestMust(t, err)
	readEquality, err := theory.MkEqualityLiteral(left, right, true)
	astTestMust(t, err)
	for _, expression := range []ast.ASTHandle{left, right, state.boolExpr[readEquality.Var()]} {
		astTestMust(t, state.markRelevant(expression))
	}
	astTestMust(t, state.AssignLiteral(readEquality.Negated(), NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	arrayEquality, err := theory.MkEqualityLiteral(a, b, true)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(arrayEquality.Negated(), NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)

	c, err := manager.MkConst(astTestSymbol(t, manager, "array-ext-trace-c"), arraySort)
	astTestMust(t, err)
	d, err := manager.MkConst(astTestSymbol(t, manager, "array-ext-trace-d"), arraySort)
	astTestMust(t, err)
	witnessEquality, err := theory.MkEqualityLiteral(c, d, true)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(witnessEquality.Negated(), NewSMTBooleanAxiom()))
	astTestMust(t, state.PropagateTheoryAssignments())
	astTestMust(t, state.PropagateTheoryEvents())
	astTestMust(t, theory.Propagate())

	outerSort, err := manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{arraySort}), boolean)
	astTestMust(t, err)
	outer, err := manager.MkConst(astTestSymbol(t, manager, "array-shared-trace-outer"), outerSort)
	astTestMust(t, err)
	shared := make([]ast.ASTHandle, 2)
	for i, name := range []string{"array-shared-trace-e", "array-shared-trace-f"} {
		shared[i], err = manager.MkConst(astTestSymbol(t, manager, name), arraySort)
		astTestMust(t, err)
		innerRead, err := manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{shared[i], index})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(innerRead, false))
		outerRead, err := manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{outer, shared[i]})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(outerRead, false))
		astTestMust(t, state.markRelevant(shared[i]))
	}
	progress, err := theory.makeInterfaceEqualities()
	astTestMust(t, err)
	if !progress {
		t.Fatal("shared array trace fixture created no interface equality")
	}
	if !tags.ShouldTrace("ext") {
		if output.Len() != 0 {
			t.Fatal("compiled-out array traces executed", output.String())
		}
		return
	}
	trace := output.String()
	for _, want := range []string{
		"[array_ext] already_diseq src/smt/theory_array_base.cpp:281",
		"[ext] assert_extensionality_core src/smt/theory_array_base.cpp:325",
		"[array_shared] collect_shared_vars src/smt/theory_array_base.cpp:422",
		"[array_shared] collect_shared_vars src/smt/theory_array_base.cpp:432",
		"[array_bug] mk_interface_eqs src/smt/theory_array_base.cpp:459",
	} {
		if !strings.Contains(trace, want) {
			t.Fatalf("missing source array trace %q in %q", want, trace)
		}
	}
}

func TestSMTArrayModelDefaultAndMergeSourceTraces(t *testing.T) {
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("array")
	manager := smtArrayTraceManager(t, tags)
	family, err := manager.RegisterArrayPlugin()
	astTestMust(t, err)
	context, _, state := smtInternalizerFixture(t, manager, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	arraySort, err := manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	astTestMust(t, err)
	value, err := manager.MkFalse()
	astTestMust(t, err)
	constant, err := manager.MkConstArray(arraySort, value)
	astTestMust(t, err)
	index, err := manager.MkTrue()
	astTestMust(t, err)
	store, err := manager.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{constant, index, value})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(store, false))
	for _, expression := range []ast.ASTHandle{constant, store} {
		astTestMust(t, state.markRelevant(expression))
	}
	generator := &SMTModelGenerator{manager: manager, context: context}
	astTestMust(t, theory.prepareArrayModel(generator))
	astTestMust(t, theory.FinalizeModel(generator))
	if !tags.ShouldTrace("array") {
		if output.Len() != 0 {
			t.Fatal("compiled-out array model traces executed", output.String())
		}
		return
	}
	trace := output.String()
	for _, want := range []string{
		"[array] set_default src/smt/theory_array_base.cpp:521",
		"[array] mg_merge src/smt/theory_array_base.cpp:568",
		"[array] collect_defaults src/smt/theory_array_base.cpp:646",
	} {
		if !strings.Contains(trace, want) {
			t.Fatalf("missing source array model trace %q in %q", want, trace)
		}
	}
}

func TestSMTFullArrayConstructorSourceTraces(t *testing.T) {
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	for _, tag := range []string{"array", "array_map_bug", "array_axiom", "mk_var_bug"} {
		tags.Enable(tag)
	}
	manager := smtArrayTraceManager(t, tags)
	family, err := manager.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, manager, nil)
	state.theoryRewriter = smtQuickTheoryRewriter(t, manager)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	arraySort, err := manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	astTestMust(t, err)
	yes, err := manager.MkTrue()
	astTestMust(t, err)
	no, err := manager.MkFalse()
	astTestMust(t, err)
	constant, err := manager.MkConstArray(arraySort, yes)
	astTestMust(t, err)
	x := smtRelevancyBool(t, manager, "array-map-trace-x")
	notX, err := manager.MkNot(x)
	astTestMust(t, err)
	mapFunction, err := manager.GetDecl(notX)
	astTestMust(t, err)
	mapped, err := manager.MkArrayMap(mapFunction, ast.SliceArguments([]ast.ASTHandle{constant}))
	astTestMust(t, err)
	asFunction, err := manager.MkFuncDecl(astTestSymbol(t, manager, "array-as-trace-function"), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	asArray, err := manager.MkAsArray(asFunction)
	astTestMust(t, err)
	store, err := manager.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{constant, yes, no})
	astTestMust(t, err)
	for _, constructor := range []ast.ASTHandle{constant, mapped, asArray, store} {
		astTestMust(t, state.Internalize(constructor, false))
		if _, err := theory.instantiateConstructorDefault(constructor); err != nil {
			t.Fatal(err)
		}
	}
	for _, constructor := range []ast.ASTHandle{constant, mapped, asArray} {
		observer, err := manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{constructor, yes})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(observer, false))
		if _, err := theory.instantiateConstructorRead(observer, constructor); err != nil {
			t.Fatal(err)
		}
	}
	if !tags.ShouldTrace("array") {
		if output.Len() != 0 {
			t.Fatal("compiled-out full array traces executed", output.String())
		}
		return
	}
	trace := output.String()
	for _, want := range []string{
		"[array] instantiate_default_const_axiom src/smt/theory_array_full.cpp:573",
		"[array] instantiate_default_map_axiom src/smt/theory_array_full.cpp:546",
		"[array] instantiate_default_as_array_axiom src/smt/theory_array_full.cpp:587",
		"[array] instantiate_default_store_axiom src/smt/theory_array_full.cpp:707",
		"[array_map_bug] instantiate_select_map_axiom src/smt/theory_array_full.cpp:481",
		"[array_map_bug] instantiate_select_map_axiom src/smt/theory_array_full.cpp:489",
		"[array] instantiate_select_map_axiom src/smt/theory_array_full.cpp:492",
		"[array_map_bug] instantiate_select_map_axiom src/smt/theory_array_full.cpp:524",
		"[array] instantiate_select_const_axiom src/smt/theory_array_full.cpp:642",
		"[array] instantiate_select_as_array_axiom src/smt/theory_array_full.cpp:678",
		"[array] try_assign_eq src/smt/theory_array_full.cpp:787",
	} {
		if !strings.Contains(trace, want) {
			t.Fatalf("missing source full-array trace %q in %q", want, trace)
		}
	}
}

func TestSMTFullArraySourceShapeGuards(t *testing.T) {
	manager := astTestManager(t)
	family, err := manager.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, manager, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	arraySort, err := manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	astTestMust(t, err)
	value, err := manager.MkTrue()
	astTestMust(t, err)
	constant, err := manager.MkConstArray(arraySort, value)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(constant, false))
	node := state.context.enodes.get(constant)
	variable := node.TheoryVar(family)
	if err = theory.addParentMap(variable, node); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-map parent accepted", err)
	}
	if err = theory.setMapInputsUpward(node); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-map upward constructor accepted", err)
	}
	if _, err = theory.instantiateConstructorRead(constant, constant); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-select observer accepted", err)
	}
}

func TestSMTOrdinaryArraySourceTraces(t *testing.T) {
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	for _, tag := range []string{"array", "array_bug", "array_axiom2b", "ext"} {
		tags.Enable(tag)
	}
	manager := smtArrayTraceManager(t, tags)
	family, err := manager.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, manager, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	theory.arrayConfig.Laziness = 0
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	arraySort, err := manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	astTestMust(t, err)
	no, err := manager.MkFalse()
	astTestMust(t, err)
	yes, err := manager.MkTrue()
	astTestMust(t, err)
	first, err := manager.MkConstArray(arraySort, no)
	astTestMust(t, err)
	second, err := manager.MkConstArray(arraySort, yes)
	astTestMust(t, err)
	store, err := manager.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{first, yes, yes})
	astTestMust(t, err)
	for _, expression := range []ast.ASTHandle{first, second, store} {
		astTestMust(t, state.Internalize(expression, false))
	}
	firstVar := state.context.enodes.get(first).TheoryVar(family)
	storeVar := state.context.enodes.get(store).TheoryVar(family)
	secondVar := state.context.enodes.get(second).TheoryVar(family)
	astTestMust(t, theory.NewEquality(firstVar, storeVar))
	read, err := manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{store, yes})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(read, false))
	_, err = theory.queueStoreReadAxiom(read, store, true)
	astTestMust(t, err)
	astTestMust(t, theory.NewDisequality(firstVar, secondVar))
	condition := smtRelevancyBool(t, manager, "array-ordinary-trace-condition")
	choice, err := manager.MkIte(condition, first, second)
	astTestMust(t, err)
	astTestMust(t, theory.Relevant(choice))
	_, err = theory.FinalCheck()
	astTestMust(t, err)

	if !tags.ShouldTrace("array") {
		if output.Len() != 0 {
			t.Fatal("compiled-out ordinary array traces executed", output.String())
		}
		return
	}
	trace := output.String()
	for _, want := range []string{
		"[array_bug] internalize_term src/smt/theory_array.cpp:267",
		"[array_bug] internalize_term_core src/smt/theory_array.cpp:241",
		"[array] mk_var src/smt/theory_array.cpp:77",
		"[array] instantiate_axiom1 src/smt/theory_array.cpp:200",
		"[array] merge_eh src/smt/theory_array.cpp:48",
		"[array] merge_eh src/smt/theory_array.cpp:62",
		"[array] set_prop_upward src/smt/theory_array_full.cpp:115",
		"[array] add_parent_select src/smt/theory_array.cpp:98",
		"[array] instantiate_axiom2a src/smt/theory_array.cpp:207",
		"[array_axiom2b] instantiate_axiom2b src/smt/theory_array.cpp:215",
		"[ext] new_diseq_eh src/smt/theory_array.cpp:308",
		"[array] instantiate_extensionality src/smt/theory_array.cpp:226",
		"[array] relevant_eh src/smt/theory_array_full.cpp:428",
		"[array] final_check_eh src/smt/theory_array.cpp:385",
	} {
		if !strings.Contains(trace, want) {
			t.Fatalf("missing ordinary array source trace %q in %q", want, trace)
		}
	}
	// Laziness 0 makes theory_array::relevant_eh return before its ite trace.
	if strings.Contains(trace, "[array] relevant_eh src/smt/theory_array.cpp:318") {
		t.Fatal("lazy-free relevant_eh traced the relevant ite")
	}
}
