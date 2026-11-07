package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTArraySharedRolesAndScopedInterfaceEqualities(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	innerSort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	astTestMust(t, err)
	outerSort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{innerSort}), boolean)
	astTestMust(t, err)
	outer, err := m.MkConst(astTestSymbol(t, m, "shared-outer"), outerSort)
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "shared-inner-a"), innerSort)
	astTestMust(t, err)
	b, err := m.MkConst(astTestSymbol(t, m, "shared-inner-b"), innerSort)
	astTestMust(t, err)
	index, err := m.MkTrue()
	astTestMust(t, err)
	for _, array := range []ast.ASTHandle{a, b} {
		read, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, index})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(read, false))
		shared, err := theory.sharedArrayNode(state.context.enodes.get(array))
		astTestMust(t, err)
		if shared {
			t.Fatal("single array role was treated as shared")
		}
		read, err = m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{outer, array})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(read, false))
		shared, err = theory.sharedArrayNode(state.context.enodes.get(array))
		astTestMust(t, err)
		if !shared {
			t.Fatal("array/index roles were not shared")
		}
		astTestMust(t, state.markRelevant(array))
	}
	for attempt := uint32(1); attempt <= 2; attempt++ {
		astTestMust(t, state.PushScope())
		status, err := theory.FinalCheck()
		astTestMust(t, err)
		if status != SMTFinalCheckContinue || theory.statistics.eqSplits != attempt {
			t.Fatal("missing interface split", status, theory.statistics.eqSplits)
		}
		equality, err := m.MkEq(a, b)
		astTestMust(t, err)
		variable, found := state.BoolVar(equality)
		if !found || !state.relevancy.IsRelevant(equality) {
			t.Fatal("interface equality was not internalized and relevant")
		}
		data, err := state.BooleanData(variable)
		astTestMust(t, err)
		if data.TryTrueFirst() {
			t.Fatal("array split acquired arithmetic's true-first preference")
		}
		status, err = theory.FinalCheck()
		astTestMust(t, err)
		if status != SMTFinalCheckDone || theory.statistics.eqSplits != attempt {
			t.Fatal("existing relevant split was repeated", status)
		}
		astTestMust(t, state.PopScopes(1))
		if _, found := state.BoolVar(equality); found {
			t.Fatal("child interface equality survived scope pop")
		}
	}
}

func TestSMTArrayLazyInterfaceFinalCheckSchedule(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	config := theory.arrayConfig
	config.LazyIEq, config.LazyIEqDelay = true, 4
	astTestMust(t, theory.ConfigureArray(config))
	for i := uint32(1); i <= 8; i++ {
		before := state.theoryProgress
		status, err := theory.FinalCheck()
		astTestMust(t, err)
		want := SMTFinalCheckContinue
		if i%4 == 0 {
			want = SMTFinalCheckDone
		}
		if status != want || theory.finalCheckIndex != i {
			t.Fatal("lazy interface schedule", i, status, want)
		}
		if want == SMTFinalCheckContinue && state.theoryProgress == before {
			t.Fatal("bounded lazy scheduling tripped the no-progress guard")
		}
	}
	astTestMust(t, state.InitializeTheorySearch())
	if theory.finalCheckIndex != 0 {
		t.Fatal("new search retained the previous lazy interface cadence")
	}
	status, err := theory.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckContinue || theory.finalCheckIndex != 1 {
		t.Fatal("new search did not restart the source delay")
	}
	config.LazyIEq = false
	astTestMust(t, theory.ConfigureArray(config))
	status, err = theory.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckDone {
		t.Fatal("disabled lazy schedule was retained")
	}
}

func TestSMTArraySharedValueAndUninterpretedParentRoles(t *testing.T) {
	for _, ufParent := range []bool{false, true} {
		m := astTestManager(t)
		family, err := m.RegisterArrayPlugin()
		astTestMust(t, err)
		_, _, state := smtInternalizerFixture(t, m, nil)
		theory, err := NewSMTArrayTheory(family)
		astTestMust(t, err)
		astTestMust(t, state.RegisterTheory(theory))
		boolean, err := m.MkBoolSort()
		astTestMust(t, err)
		sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
		astTestMust(t, err)
		a, err := m.MkConst(astTestSymbol(t, m, "shared-value-array"), sort)
		astTestMust(t, err)
		index, err := m.MkTrue()
		astTestMust(t, err)
		read, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, index})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(read, false))
		if ufParent {
			function, err := m.MkFuncDecl(astTestSymbol(t, m, "shared-array-predicate"), []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			parent, err := m.MkApp(function, []ast.ASTHandle{a})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(parent, false))
		} else {
			outerSort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), sort)
			astTestMust(t, err)
			parent, err := m.MkConstArray(outerSort, a)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(parent, false))
		}
		shared, err := theory.sharedArrayNode(state.context.enodes.get(a))
		astTestMust(t, err)
		if !shared {
			t.Fatal("shared array value or uninterpreted parent was missed", ufParent)
		}
	}
}
