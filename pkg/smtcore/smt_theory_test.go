package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type smtInternalizerTestTheory struct {
	*SMTTheoryBase
	atoms, equalities, terms, sortConstraints int
	newEqualities, newDisequalities           []SMTTheoryPropagation
	assignments                               []struct {
		variable int32
		value    bool
	}
	canPropagate, propagationCalled bool
	relevant                        []ast.ASTHandle
	setupCalls, searchCalls         int
	restartCalls, finalCheckCalls   int
	finalizeModelCalls              int
	finalStatus                     SMTFinalCheckStatus
}

func (t *smtInternalizerTestTheory) DefaultInternalizer() bool { return false }

func (t *smtInternalizerTestTheory) InternalizeAtom(expression ast.ASTHandle, gateContext bool) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	for index := uint32(0); index < count; index++ {
		argument, err := state.manager.GetArg(expression, index)
		if err != nil {
			return false, err
		}
		if err = state.Internalize(argument, false); err != nil {
			return false, err
		}
	}
	variable, err := state.MkBoolVar(expression)
	if err != nil {
		return false, err
	}
	if err = state.SetVariableTheory(variable, t.FamilyID()); err != nil {
		return false, err
	}
	t.atoms++
	return true, nil
}

func (t *smtInternalizerTestTheory) InternalizeEquality(ast.ASTHandle, int32) error {
	t.equalities++
	return nil
}

func (t *smtInternalizerTestTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	node, err := state.MkEnode(expression, true, false, false)
	if err != nil {
		return false, err
	}
	if _, err = t.CreateVariable(node); err != nil {
		return false, err
	}
	t.terms++
	return true, nil
}

func (t *smtInternalizerTestTheory) ApplySortConstraint(*SMTEnode, ast.ASTHandle) error {
	t.sortConstraints++
	return nil
}

func (t *smtInternalizerTestTheory) NewEquality(left, right SMTTheoryVar) error {
	t.newEqualities = append(t.newEqualities, SMTTheoryPropagation{Theory: t.FamilyID(), Left: left, Right: right})
	return nil
}

func (t *smtInternalizerTestTheory) NewDisequality(left, right SMTTheoryVar) error {
	t.newDisequalities = append(t.newDisequalities, SMTTheoryPropagation{Theory: t.FamilyID(), Left: left, Right: right})
	return nil
}

func (t *smtInternalizerTestTheory) Assign(variable int32, value bool) error {
	t.assignments = append(t.assignments, struct {
		variable int32
		value    bool
	}{variable: variable, value: value})
	return nil
}

func (t *smtInternalizerTestTheory) CanPropagate() bool { return t.canPropagate }

func (t *smtInternalizerTestTheory) Propagate() error {
	t.propagationCalled = true
	t.canPropagate = false
	return nil
}

func (t *smtInternalizerTestTheory) Relevant(expression ast.ASTHandle) error {
	t.relevant = append(t.relevant, expression)
	return nil
}

func (t *smtInternalizerTestTheory) Setup() error {
	t.setupCalls++
	return nil
}

func (t *smtInternalizerTestTheory) InitializeSearch() error {
	t.searchCalls++
	return nil
}

func (t *smtInternalizerTestTheory) Restart() error {
	t.restartCalls++
	return nil
}

func (t *smtInternalizerTestTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	t.finalCheckCalls++
	return t.finalStatus, nil
}

func (t *smtInternalizerTestTheory) FinalizeModel(*SMTModelGenerator) error {
	t.finalizeModelCalls++
	return nil
}

func TestSMTTheoryInternalizationCallbacksVariablesAndScopes(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	base, err := NewSMTTheoryBase(family)
	astTestMust(t, err)
	theory := &smtInternalizerTestTheory{SMTTheoryBase: base}
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	astTestMust(t, state.InitializeTheorySearch())
	astTestMust(t, state.RestartTheories())
	if theory.setupCalls != 1 || theory.searchCalls != 1 || theory.restartCalls != 1 {
		t.Fatal("theory setup/search lifecycle", theory.setupCalls, theory.searchCalls, theory.restartCalls)
	}

	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	termInfo, err := m.NewFuncDeclInfo(family, 9001, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	termDecl, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "theory-term"), nil, integer, termInfo)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&termInfo))
	term, err := m.MkApp(termDecl, nil)
	astTestMust(t, err)
	atomInfo, err := m.NewFuncDeclInfo(family, 9002, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	atomDecl, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "theory-atom"), []ast.ASTHandle{integer}, boolean, atomInfo)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&atomInfo))
	atom, err := m.MkApp(atomDecl, []ast.ASTHandle{term})
	astTestMust(t, err)

	astTestMust(t, state.PushScope())
	astTestMust(t, state.Internalize(atom, true))
	node := state.context.enodes.get(term)
	if node == nil || theory.atoms != 1 || theory.terms != 1 || theory.sortConstraints != 1 || theory.NumVariables() != 1 || !theory.IsAttachedToVariable(node) {
		t.Fatal("theory atom/term internalization", theory.atoms, theory.terms, theory.sortConstraints, theory.NumVariables(), node)
	}
	atomLiteral, err := state.Literal(atom)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(atomLiteral, NewSMTBooleanAxiom()))
	astTestMust(t, state.markRelevant(atom))
	theory.canPropagate = true
	consistent, err := state.PropagateCore()
	astTestMust(t, err)
	if !consistent || !theory.propagationCalled {
		t.Fatal("theory propagation fixed point", consistent, theory.propagationCalled)
	}
	if len(theory.assignments) != 1 || theory.assignments[0].variable != atomLiteral.Var() || !theory.assignments[0].value {
		t.Fatal("theory assignment callback", theory.assignments)
	}
	if len(theory.relevant) != 2 || theory.relevant[0] != atom || theory.relevant[1] != term {
		t.Fatal("theory relevancy callback", theory.relevant)
	}
	theory.finalStatus = SMTFinalCheckGiveUp
	status, err := state.FinalCheckTheories()
	astTestMust(t, err)
	if status != SMTFinalCheckGiveUp || len(state.IncompleteTheories()) != 1 {
		t.Fatal("theory give-up final check", status, state.IncompleteTheories())
	}
	theory.finalStatus = SMTFinalCheckContinue
	status, err = state.FinalCheckTheories()
	astTestMust(t, err)
	if status != SMTFinalCheckContinue || theory.finalCheckCalls != 2 || state.Statistics().NumFinalChecks != 2 {
		t.Fatal("theory continuing final check", status, theory.finalCheckCalls, state.Statistics())
	}
	// Native equality-propagation reasons require distinct operand enodes.
	// Use a distinct term to exercise the theory equality callback.
	other, err := m.MkConst(astTestSymbol(t, m, "theory-equality-other"), integer)
	astTestMust(t, err)
	equality, err := m.MkEq(term, other)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(equality, true))
	if theory.equalities != 1 {
		t.Fatal("theory equality callback", theory.equalities)
	}
	astTestMust(t, state.PopScopes(1))
	if theory.NumVariables() != 0 || node.TheoryVar(family) != SMTNullTheoryVar || state.context.enodes.get(term) != nil {
		t.Fatal("theory scope rollback", theory.NumVariables(), node.TheoryVar(family), state.context.enodes.get(term))
	}
}

func TestSMTTheoryAttachmentQueuesEqualityAndDisequalityEvents(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	base, err := NewSMTTheoryBase(family)
	astTestMust(t, err)
	theory := &smtInternalizerTestTheory{SMTTheoryBase: base}
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	astTestMust(t, state.PushScope())

	_, root := smtEnodeConstant(t, m, 2260, integer, 0)
	_, member := smtEnodeConstant(t, m, 2261, integer, 0)
	root.root, root.next, root.classSize = root, member, 2
	member.root, member.next = root, root
	rootVariable, err := theory.CreateVariable(root)
	astTestMust(t, err)
	memberVariable, err := theory.CreateVariable(member)
	astTestMust(t, err)
	pendingEqualities := state.PendingTheoryEqualities()
	if len(pendingEqualities) != 1 || pendingEqualities[0].Left != rootVariable || pendingEqualities[0].Right != memberVariable {
		t.Fatal("theory equality queue", pendingEqualities)
	}
	astTestMust(t, state.PropagateTheoryEvents())
	if len(theory.newEqualities) != 1 || len(state.PendingTheoryEqualities()) != 0 {
		t.Fatal("theory equality delivery", theory.newEqualities, state.PendingTheoryEqualities())
	}

	leftOwner, _ := smtEnodeConstant(t, m, 2262, integer, 0)
	rightOwner, _ := smtEnodeConstant(t, m, 2263, integer, 0)
	astTestMust(t, state.Internalize(leftOwner, false))
	astTestMust(t, state.Internalize(rightOwner, false))
	leftNode, rightNode := state.context.enodes.get(leftOwner), state.context.enodes.get(rightOwner)
	equality, err := m.MkEq(leftOwner, rightOwner)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(equality, true))
	equalityLiteral, err := state.Literal(equality)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(equalityLiteral.Negated(), NewSMTBooleanAxiom()))
	rightVariable, err := theory.CreateVariable(rightNode)
	astTestMust(t, err)
	leftVariable, err := theory.CreateVariable(leftNode)
	astTestMust(t, err)
	pendingDisequalities := state.PendingTheoryDisequalities()
	if len(pendingDisequalities) != 1 || pendingDisequalities[0].Left != leftVariable || pendingDisequalities[0].Right != rightVariable {
		t.Fatal("theory disequality queue", pendingDisequalities)
	}
	astTestMust(t, state.PropagateTheoryEvents())
	if len(theory.newDisequalities) != 1 || len(state.PendingTheoryDisequalities()) != 0 {
		t.Fatal("theory disequality delivery", theory.newDisequalities, state.PendingTheoryDisequalities())
	}

	astTestMust(t, state.PopScopes(1))
	if theory.NumVariables() != 0 || root.TheoryVar(family) != SMTNullTheoryVar || member.TheoryVar(family) != SMTNullTheoryVar || len(state.PendingTheoryEqualities()) != 0 || len(state.PendingTheoryDisequalities()) != 0 {
		t.Fatal("theory event scope rollback")
	}
}

func TestSMTTheoryBaseConstructsEqualityLiteral(t *testing.T) {
	m := astTestManager(t)
	_, values := smtUFConstants(t, m, "theory-equality-literal-", 2)
	_, _, state := smtInternalizerFixture(t, m, nil)
	family, err := m.GetFamilyID(values[0])
	astTestMust(t, err)
	if family == ast.NullFamilyID {
		family = 71
	}
	base, err := NewSMTTheoryBase(family)
	astTestMust(t, err)
	astTestMust(t, base.Initialize(state))
	literal, err := base.MkEqualityLiteral(values[0], values[1], false)
	astTestMust(t, err)
	if literal == SMTNullLiteral || literal.Sign() {
		t.Fatal("equality literal", literal)
	}
	equality, err := m.MkEq(values[0], values[1])
	astTestMust(t, err)
	variable, found := state.BoolVar(equality)
	if !found || variable != literal.Var() || state.context.enodes.get(values[0]) == nil || state.context.enodes.get(values[1]) == nil {
		t.Fatal("equality internalization", variable, found, literal)
	}
	reversed, err := base.MkEqualityLiteral(values[1], values[0], true)
	astTestMust(t, err)
	if reversed != literal {
		t.Fatal("reversed search equality created a second Boolean atom", literal, reversed)
	}
}
