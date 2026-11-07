package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtSequenceInterfaceFixture struct {
	m          *ast.ASTManager
	theory     *SMTSequenceTheory
	state      *SMTInternalizerState
	sort, x, y ast.ASTHandle
}

func newSMTSequenceInterfaceFixture(t *testing.T) smtSequenceInterfaceFixture {
	t.Helper()
	m := astTestManager(t)
	witness, err := m.MkString("interface-sort")
	astTestMust(t, err)
	sort, err := m.GetSort(witness)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "interface-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "interface-y"), sort)
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	astTestMust(t, state.Internalize(x, false))
	astTestMust(t, state.Internalize(y, false))
	return smtSequenceInterfaceFixture{m, theory, state, sort, x, y}
}

func (f smtSequenceInterfaceFixture) parent(t *testing.T, word ast.ASTHandle) {
	t.Helper()
	boolean, err := f.m.MkBoolSort()
	astTestMust(t, err)
	declaration, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "interface-parent"), []ast.ASTHandle{f.sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	parent, err := f.m.MkApp(declaration, []ast.ASTHandle{word})
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(parent, false))
}

func TestSMTSequenceExtensionalityRequiresSharingAndPrefersTrue(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	astTestMust(t, f.state.context.SetRelevant(f.x, true))
	astTestMust(t, f.state.context.SetRelevant(f.y, true))
	progress, err := f.theory.checkSequenceExtensionality()
	astTestMust(t, err)
	if progress {
		t.Fatal("unshared sequence roots acquired an interface split")
	}
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, f.state.PushScope())
		f.parent(t, f.y)
		progress, err = f.theory.checkSequenceExtensionality()
		astTestMust(t, err)
		equality, err := f.m.MkEq(f.x, f.y)
		astTestMust(t, err)
		variable, found := f.state.BoolVar(equality)
		if !progress || !found {
			t.Fatal("shared sequence root did not create the canonical interface equality")
		}
		data, err := f.state.BooleanData(variable)
		astTestMust(t, err)
		value, err := f.state.Assignment(NewSMTLiteral(variable, false))
		astTestMust(t, err)
		if !data.TryTrueFirst() || value != lbool.Undef || !f.state.isRelevant(equality) || f.state.context.enodes.get(f.x).root == f.state.context.enodes.get(f.y).root {
			t.Fatal("interface equality was not an undecided true-first relevant split")
		}
		astTestMust(t, f.state.PopScopes(1))
		if progress, err := f.theory.checkSequenceExtensionality(); err != nil || progress {
			t.Fatal("foreign-parent sharing survived scope pop", progress, err)
		}
	}
}

func TestSMTSequenceExtensionalityKeepsEarlierUnmarkedCandidate(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	f.parent(t, f.y)
	astTestMust(t, f.state.context.SetRelevant(f.y, true))
	progress, err := f.theory.checkSequenceExtensionality()
	astTestMust(t, err)
	if !progress {
		t.Fatal("an earlier unmarked root was filtered out of source candidates")
	}
	equality, err := f.m.MkEq(f.x, f.y)
	astTestMust(t, err)
	if _, found := f.state.BoolVar(equality); !found {
		t.Fatal("source candidate order did not select the earlier unmarked root")
	}
}

func TestSMTSequenceExtensionalityCachesCanonicalConflictInScope(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	f.parent(t, f.y)
	astTestMust(t, f.state.context.SetRelevant(f.y, true))
	a, err := f.m.MkString("a")
	astTestMust(t, err)
	b, err := f.m.MkString("b")
	astTestMust(t, err)
	astTestMust(t, f.theory.solutions.Update(f.x, a, nil))
	astTestMust(t, f.theory.solutions.Update(f.y, b, nil))
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, f.state.PushScope())
		progress, err := f.theory.checkSequenceExtensionality()
		astTestMust(t, err)
		if progress || !f.theory.exclusions.Contains(f.x, f.y) {
			t.Fatal("inconsistent canonical words were split instead of excluded")
		}
		astTestMust(t, f.state.PopScopes(1))
		if f.theory.exclusions.Contains(f.x, f.y) {
			t.Fatal("canonical conflict exclusion survived its scope")
		}
	}
}

func TestSMTSequenceExtensionalityHonorsDisabledRelevancy(t *testing.T) {
	m := astTestManager(t)
	value, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "unmarked-sequence-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "unmarked-sequence-y"), sort)
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	context, relevancy, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	astTestMust(t, state.Internalize(x, false))
	astTestMust(t, state.Internalize(y, false))
	equality, err := m.MkEq(x, y)
	astTestMust(t, err)
	astTestPin(t, m, equality)
	smtSequenceInterfaceFixture{m: m, theory: theory, state: state, sort: sort}.parent(t, y)
	for _, enabled := range []bool{true, false, true} {
		relevancy.enabled = enabled
		astTestMust(t, state.PushScope())
		progress, err := theory.checkSequenceExtensionality()
		astTestMust(t, err)
		if progress == enabled {
			t.Fatal("extensionality ignored the current relevance policy", enabled, progress)
		}
		_, found := state.BoolVar(equality)
		if found == enabled {
			t.Fatal("interface equality was not scoped to the disabled-filter pass", enabled, found)
		}
		if context.relevant.get(x) || context.relevant.get(y) {
			t.Fatal("querying the relevance policy changed its marks")
		}
		astTestMust(t, state.PopScopes(1))
		if _, found := state.BoolVar(equality); found {
			t.Fatal("extensionality equality survived scope rollback")
		}
	}
}

func TestSMTSequenceExtensionalityHonorsReducedSideAndResidualExclusions(t *testing.T) {
	for _, mode := range []string{"unit-elements", "word-residue"} {
		t.Run(mode, func(t *testing.T) {
			f := newSMTSequenceInterfaceFixture(t)
			f.parent(t, f.y)
			astTestMust(t, f.state.context.SetRelevant(f.y, true))
			componentSort := f.sort
			if mode == "unit-elements" {
				var err error
				componentSort, _, err = f.m.SequenceSortElement(f.sort)
				astTestMust(t, err)
			}
			var components, values []ast.ASTHandle
			for _, name := range []string{"excluded-first", "excluded-second"} {
				component, err := f.m.MkConst(astTestSymbol(t, f.m, name), componentSort)
				astTestMust(t, err)
				components = append(components, component)
				var value ast.ASTHandle
				if mode == "unit-elements" {
					value, err = f.m.MkSequenceUnit(component)
				} else {
					var suffix ast.ASTHandle
					suffix, err = f.m.MkString("suffix")
					astTestMust(t, err)
					value, err = f.m.MkSequenceConcat(component, suffix)
				}
				astTestMust(t, err)
				values = append(values, value)
			}
			astTestMust(t, f.theory.solutions.Update(f.x, values[0], nil))
			astTestMust(t, f.theory.solutions.Update(f.y, values[1], nil))
			equality, err := f.m.MkEq(f.x, f.y)
			astTestMust(t, err)
			astTestPin(t, f.m, equality)
			for attempt := 0; attempt < 2; attempt++ {
				astTestMust(t, f.state.PushScope())
				astTestMust(t, f.theory.exclusions.Update(components[0], components[1]))
				progress, err := f.theory.checkSequenceExtensionality()
				astTestMust(t, err)
				if progress || f.theory.exclusions.Contains(f.x, f.y) {
					t.Fatal("a reduced excluded pair was split or cached as an unconditional word conflict")
				}
				if _, found := f.state.BoolVar(equality); found {
					t.Fatal("reduced exclusion allocated an interface atom")
				}
				astTestMust(t, f.state.PopScopes(1))
				astTestMust(t, f.state.PushScope())
				progress, err = f.theory.checkSequenceExtensionality()
				astTestMust(t, err)
				if _, found := f.state.BoolVar(equality); !progress || !found {
					t.Fatal("popped reduced exclusion still blocked the interface atom")
				}
				astTestMust(t, f.state.PopScopes(1))
			}
		})
	}
}

func TestSMTSequenceExtensionalityRetainsCanonicalAcrossCandidates(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	z, err := f.m.MkConst(astTestSymbol(t, f.m, "interface-z"), f.sort)
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(z, false))
	f.parent(t, z)
	astTestMust(t, f.state.context.SetRelevant(z, true))
	var parts []ast.ASTHandle
	for _, name := range []string{"canonical-u", "canonical-v", "canonical-w"} {
		part, err := f.m.MkConst(astTestSymbol(t, f.m, name), f.sort)
		astTestMust(t, err)
		parts = append(parts, part)
	}
	prefix, err := f.m.MkString("A")
	astTestMust(t, err)
	different, err := f.m.MkString("B")
	astTestMust(t, err)
	first, err := f.m.MkSequenceConcat(prefix, parts[0])
	astTestMust(t, err)
	leftAssociated, err := f.m.MkSequenceConcat(first, parts[1])
	astTestMust(t, err)
	yValue, err := f.m.MkSequenceConcat(prefix, parts[2])
	astTestMust(t, err)
	astTestMust(t, f.theory.solutions.Update(f.x, different, nil))
	astTestMust(t, f.theory.solutions.Update(f.y, yValue, nil))
	astTestMust(t, f.theory.solutions.Update(z, leftAssociated, nil))
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, f.state.PushScope())
		progress, err := f.theory.checkSequenceExtensionality()
		astTestMust(t, err)
		equality, err := f.m.MkEq(f.y, z)
		astTestMust(t, err)
		if _, found := f.state.BoolVar(equality); !progress || !found || !f.theory.exclusions.Contains(z, f.x) {
			t.Fatal("canonical owner did not survive rejection of the first candidate")
		}
		astTestMust(t, f.state.PopScopes(1))
	}
}
