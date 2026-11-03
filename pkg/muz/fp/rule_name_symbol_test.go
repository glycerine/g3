package fp

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type fpRuleNameFixture struct {
	t     *testing.T
	m     *ast.ASTManager
	e     *PropositionalEngine
	roots []ast.ASTRef
}

func newFPRuleNameFixture(t *testing.T) *fpRuleNameFixture {
	t.Helper()
	m, globals := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	fragmentMust(t, err)
	f := &fpRuleNameFixture{t: t, m: m, e: e}
	t.Cleanup(func() {
		fragmentMust(t, e.Close())
		fragmentMust(t, closeASTRefs(m, f.roots))
	})
	return f
}

func (f *fpRuleNameFixture) own(h ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	fragmentMust(f.t, err)
	ref, err := f.m.NewRef(h)
	fragmentMust(f.t, err)
	f.roots = append(f.roots, ref)
	return h
}

func (f *fpRuleNameFixture) predicate(name string, args ...ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle) {
	f.t.Helper()
	symbol, err := f.m.Symbols.Intern(name)
	fragmentMust(f.t, err)
	domain := make([]ast.ASTHandle, len(args))
	for i, arg := range args {
		domain[i], err = f.m.GetSort(arg)
		fragmentMust(f.t, err)
	}
	boolean := f.own(f.m.MkBoolSort())
	decl := f.own(f.m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{}))
	fragmentMust(f.t, f.e.RegisterPredicate(decl))
	return decl, f.own(f.m.MkApp(decl, args))
}

func fpRuleNameSymbols(t *testing.T, m *ast.ASTManager) []usymbol.Symbol {
	t.Helper()
	empty, err := m.Symbols.Intern("")
	fragmentMust(t, err)
	numericText, err := m.Symbols.Intern("k!7")
	fragmentMust(t, err)
	return []usymbol.Symbol{{}, empty, usymbol.Numerical(7), numericText}
}

func checkFPRuleNameSnapshot(t *testing.T, e *PropositionalEngine, want []usymbol.Symbol) []ast.ASTRef {
	t.Helper()
	forms, names, err := e.NamedRulesWithSymbols()
	fragmentMust(t, err)
	if len(forms) != len(want) || !reflect.DeepEqual(names, want) {
		fragmentMust(t, closeASTRefs(e.manager, forms))
		t.Fatal("typed normalized names", len(forms), names, want)
	}
	return forms
}

func TestFixedpointRuleNameSymbolsUpdateDistinctIdentities(t *testing.T) {
	f := newFPRuleNameFixture(t)
	names := fpRuleNameSymbols(t, f.m)
	_, gate := f.predicate("typed-update-gate")
	heads := make([]ast.ASTHandle, len(names))
	decls := make([]ast.ASTHandle, len(names))
	for i, name := range names {
		decls[i], heads[i] = f.predicate(fmt.Sprintf("typed-update-head-%d", i))
		fragmentMust(t, f.e.AddRuleWithSymbolAndBound(heads[i], name, uint32(31+i)))
	}
	fragmentMust(t, f.e.flushPendingRules())
	fragmentMust(t, closeASTRefs(f.m, checkFPRuleNameSnapshot(t, f.e, names)))
	for updated, name := range names {
		stronger := f.own(f.m.MkImplies(gate, heads[updated]))
		fragmentMust(t, f.e.UpdateRuleWithSymbol(stronger, name))
		count, err := f.e.set.Len()
		fragmentMust(t, err)
		if count != len(names) {
			t.Fatal("typed update added or removed another identity", count)
		}
		seen := make(map[usymbol.Symbol]bool)
		for i := 0; i < count; i++ {
			rule, err := f.e.set.Rule(i)
			fragmentMust(t, err)
			got, err := rule.NameSymbol()
			fragmentMust(t, err)
			head, err := rule.HeadDeclaration()
			fragmentMust(t, err)
			size, err := rule.TailSize()
			fragmentMust(t, err)
			match := -1
			for j, d := range decls {
				if head == d {
					match = j
					break
				}
			}
			if match < 0 || got != names[match] || seen[got] {
				fragmentMust(t, rule.Close())
				t.Fatal("update conflated numeric/text or null/empty labels", head, got)
			}
			seen[got] = true
			wantSize := 0
			if match <= updated {
				wantSize = 1
			}
			if size != wantSize {
				t.Fatal("typed update selected wrong rule", match, size, wantSize)
			}
			if size == 1 {
				tail, negative, err := rule.Tail(0)
				fragmentMust(t, err)
				if negative || tail != gate {
					t.Fatal("updated rule lost strengthening gate", tail, negative)
				}
			}
			fragmentMust(t, rule.Close())
		}
		forms, rawNames, bounds, err := f.e.RawRulesWithSymbols()
		fragmentMust(t, err)
		if !reflect.DeepEqual(rawNames, names) || !reflect.DeepEqual(bounds, []uint32{31, 32, 33, 34}) {
			t.Fatal("update changed raw metadata", rawNames, bounds)
		}
		for i, ref := range forms {
			h, err := f.m.RefGet(ref)
			fragmentMust(t, err)
			if h != heads[i] {
				t.Fatal("update replaced original raw formula")
			}
		}
		fragmentMust(t, closeASTRefs(f.m, forms))
	}
}

func TestFixedpointRuleNameSymbolsPreserveRawScopesAndOpenPromotion(t *testing.T) {
	f := newFPRuleNameFixture(t)
	names := fpRuleNameSymbols(t, f.m)
	boolean := f.own(f.m.MkBoolSort())
	x := f.own(f.m.MkVar(0, boolean))
	binderName, err := f.m.Symbols.Intern("typed-scope-x")
	fragmentMust(t, err)
	var closed []ast.ASTHandle
	for i, name := range names {
		_, head := f.predicate(fmt.Sprintf("typed-closed-%d", i), x)
		formula := f.own(f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{binderName}, Body: head}))
		closed = append(closed, formula)
		fragmentMust(t, f.e.AddRuleWithSymbolAndBound(formula, name, uint32(40+i)))
	}
	before, namesBefore, boundsBefore, err := f.e.RawRulesWithSymbols()
	fragmentMust(t, err)
	defer func() { fragmentMust(t, closeASTRefs(f.m, before)) }()
	if !reflect.DeepEqual(namesBefore, names) || !reflect.DeepEqual(boundsBefore, []uint32{40, 41, 42, 43}) {
		t.Fatal("closed raw metadata", namesBefore, boundsBefore)
	}
	fragmentMust(t, f.e.Push())
	for i, name := range names {
		_, head := f.predicate(fmt.Sprintf("typed-open-%d", i), x)
		fragmentMust(t, f.e.AddRuleWithSymbolAndBound(head, name, uint32(50+i)))
	}
	scoped, scopedNames, scopedBounds, err := f.e.RawRulesWithSymbols()
	fragmentMust(t, err)
	defer func() { fragmentMust(t, closeASTRefs(f.m, scoped)) }()
	expectedNames := append(append([]usymbol.Symbol(nil), names...), names...)
	if len(scoped) != 8 || !reflect.DeepEqual(scopedNames, expectedNames) || !reflect.DeepEqual(scopedBounds, []uint32{40, 41, 42, 43, 50, 51, 52, 53}) {
		t.Fatal("scoped raw metadata", len(scoped), scopedNames, scopedBounds)
	}
	forms, err := f.e.RuleFormulas()
	fragmentMust(t, err)
	if len(forms) != 8 {
		t.Fatal("open promotion lost normalized or pending rules", len(forms))
	}
	fragmentMust(t, closeASTRefs(f.m, forms))
	promoted, remainingNames, remainingBounds, err := f.e.RawRulesWithSymbols()
	fragmentMust(t, err)
	if !reflect.DeepEqual(remainingNames, names) || !reflect.DeepEqual(remainingBounds, boundsBefore) {
		t.Fatal("open promotion changed closed raw metadata", remainingNames, remainingBounds)
	}
	fragmentMust(t, closeASTRefs(f.m, promoted))
	fragmentMust(t, f.e.Pop())
	restored, restoredNames, restoredBounds, err := f.e.RawRulesWithSymbols()
	fragmentMust(t, err)
	if !reflect.DeepEqual(restoredNames, namesBefore) || !reflect.DeepEqual(restoredBounds, boundsBefore) || len(restored) != len(closed) {
		t.Fatal("pop changed source archive", restoredNames, restoredBounds)
	}
	for i, ref := range restored {
		h, err := f.m.RefGet(ref)
		fragmentMust(t, err)
		if h != closed[i] {
			t.Fatal("pop replaced original closed formula")
		}
	}
	fragmentMust(t, closeASTRefs(f.m, restored))
	fragmentMust(t, f.e.Close())
	for _, ref := range append(before, scoped...) {
		if _, err := f.m.RefGet(ref); err != nil {
			t.Fatal("held raw snapshot expired with source context", err)
		}
	}
}

func TestFixedpointRuleNameSymbolsSurviveActualQueryClone(t *testing.T) {
	f := newFPRuleNameFixture(t)
	names := fpRuleNameSymbols(t, f.m)
	for i, name := range names {
		_, head := f.predicate(fmt.Sprintf("typed-clone-%d", i))
		fragmentMust(t, f.e.AddRuleWithSymbol(head, name))
	}
	fragmentMust(t, f.e.flushPendingRules())
	clone, err := f.e.newQueryClone()
	fragmentMust(t, err)
	defer func() { fragmentMust(t, clone.Close()) }()
	forms := checkFPRuleNameSnapshot(t, clone, names)
	defer func() { fragmentMust(t, closeASTRefs(f.m, forms)) }()
	fragmentMust(t, clone.flushPendingRules())
	fragmentMust(t, closeASTRefs(f.m, checkFPRuleNameSnapshot(t, clone, names)))
	fragmentMust(t, f.e.Close())
	fragmentMust(t, closeASTRefs(f.m, checkFPRuleNameSnapshot(t, clone, names)))
	for _, ref := range forms {
		if _, err := f.m.RefGet(ref); err != nil {
			t.Fatal("query clone snapshot lost its owner", err)
		}
	}
}

func TestFixedpointRuleNameSymbolsRefuseInvalidOwnersBeforePhaseChanges(t *testing.T) {
	f := newFPRuleNameFixture(t)
	_, head := f.predicate("typed-guard-head")
	fragmentMust(t, f.e.AddRule(head, ""))
	fragmentMust(t, closeASTRefs(f.m, checkFPRuleNameSnapshot(t, f.e, []usymbol.Symbol{{}})))
	finalized, err := f.e.set.Finalize()
	fragmentMust(t, err)
	if !finalized {
		t.Fatal("guard fixture did not finalize")
	}
	other := newFPRuleNameFixture(t)
	foreign, err := other.m.Symbols.Intern("foreign-rule-name")
	fragmentMust(t, err)
	local, err := f.m.Symbols.Intern("marked-rule-name")
	fragmentMust(t, err)
	marked, err := local.Mark()
	fragmentMust(t, err)
	for _, name := range []usymbol.Symbol{foreign, marked} {
		before := f.m.Stats()
		revision, live := f.e.revision, f.e.set
		rawBefore := append([]rawRuleFormula(nil), f.e.rawRules...)
		for _, operation := range []func() error{
			func() error { return f.e.AddRuleWithSymbol(head, name) },
			func() error { return f.e.AddRuleWithSymbolAndBound(head, name, 77) },
			func() error { return f.e.UpdateRuleWithSymbol(head, name) },
		} {
			if err := operation(); err == nil {
				t.Fatal("invalid symbol name accepted")
			}
			got, err := f.e.set.IsFinalized()
			fragmentMust(t, err)
			if !got || f.e.set != live || f.e.revision != revision || !reflect.DeepEqual(f.e.rawRules, rawBefore) {
				t.Fatal("invalid name changed rule phase or metadata")
			}
		}
		after := f.m.Stats()
		if before.LiveNodes != after.LiveNodes || before.LiveReferences != after.LiveReferences {
			t.Fatal("invalid names changed AST ownership", before, after)
		}
	}
}

func TestFixedpointRuleNameSymbolsPendingCurrentParameters(t *testing.T) {
	m, globals := newPDRLifecycleServices(t)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(globals.SourceSetName("fixedpoint.xform.fix_unbound_vars", "false"))
	e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	must(err)
	t.Cleanup(func() { must(e.Close()) })
	var roots []ast.ASTRef
	t.Cleanup(func() { must(closeASTRefs(m, roots)) })
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(text string) ast.ASTHandle {
		symbol, err := m.Symbols.Intern(text)
		must(err)
		declaration, err := m.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		return declaration
	}
	p, opaque := declare("typed-pending-head"), declare("typed-pending-opaque")
	must(e.RegisterPredicate(p))
	x, err := m.MkVar(0, boolean)
	must(err)
	y, err := m.MkVar(1, boolean)
	must(err)
	head, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	tail, err := m.MkApp(opaque, []ast.ASTHandle{y})
	must(err)
	body, err := m.MkImplies(tail, head)
	must(err)
	formula, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean, boolean}, Names: []usymbol.Symbol{usymbol.Numerical(1), usymbol.Numerical(0)}, Body: body})
	must(err)
	root, err := m.NewRef(formula)
	must(err)
	roots = append(roots, root)
	names := fpRuleNameSymbols(t, m)
	for i, name := range names {
		if i%2 == 0 {
			must(e.AddRuleWithSymbol(formula, name))
		} else {
			must(e.AddRuleWithSymbolAndBound(formula, name, uint32(i+7)))
		}
	}
	readTail := func(i int) ast.ASTHandle {
		rule, err := e.set.Rule(i)
		must(err)
		defer func() { must(rule.Close()) }()
		name, err := rule.NameSymbol()
		must(err)
		if name != names[i] {
			t.Fatal("normalization changed typed rule identity", i, name, names[i])
		}
		tail, negative, err := rule.Tail(0)
		must(err)
		if negative {
			t.Fatal("normalization negated the interpreted tail")
		}
		return tail
	}
	for i := range names {
		declaration, err := m.GetDecl(readTail(i))
		must(err)
		if declaration != opaque {
			t.Fatal("initial disabled option changed the unbound tail")
		}
	}
	local := globals.Context.EmptyValue()
	key := "xform.fix_unbound_vars"
	must(globals.Context.SetBoolName(&local, &key, true))
	must(e.SetQueryParameters(&local))
	must(globals.Context.Del(&local))
	must(e.flushPendingRules())
	forms, gotNames, err := e.NamedRulesWithSymbols()
	roots = append(roots, forms...)
	must(err)
	if len(forms) != len(names) || len(gotNames) != len(names) {
		t.Fatal("pending flush changed the number of typed rules")
	}
	stable := make([]ast.ASTHandle, len(names))
	for i, name := range names {
		stable[i] = readTail(i)
		op, err := m.GetDeclKind(stable[i])
		must(err)
		if gotNames[i] != name || op != ast.ASTOpIff {
			t.Fatal("flush did not apply the current option with the original name", i, gotNames[i], op)
		}
		quantified, err := m.GetArg(stable[i], 0)
		must(err)
		if kind, err := m.GetKind(quantified); err != nil || kind != ast.ASTQuantifier {
			t.Fatal("current option omitted its quantifier", kind, err)
		}
		if forall, err := m.IsForall(quantified); err != nil || forall {
			t.Fatal("current option did not existentially bind the unbound variable", forall, err)
		}
	}
	must(e.SetQueryParameters(nil))
	for repeat := 0; repeat < 2; repeat++ {
		must(e.flushPendingRules())
		for i := range names {
			if readTail(i) != stable[i] {
				t.Fatal("a repeated flush renormalized an already flushed rule", repeat, i)
			}
		}
	}
}
