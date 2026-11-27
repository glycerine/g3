package smtcore

// Source: src/smt/smt_enode.h and src/smt/smt_enode.cpp. This file contains
// the context-independent enode representation and algorithms.

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/approx"
)

type SMTTransJustification struct {
	Target        *SMTEnode
	Justification SMTEqualityJustification
}

type SMTEnodeOptions struct {
	Generation           uint32
	SuppressArguments    bool
	MergeTruthValue      bool
	InternalScopeLevel   uint32
	CongruenceEnabled    bool
	UpdateChildrenParent bool
}

// SMTEnode is the concrete congruence-closure node. Owner and argument nodes
// are borrowed from the enclosing SMT context.
type SMTEnode struct {
	manager                *ast.ASTManager
	root, next, congruence *SMTEnode
	arguments              []*SMTEnode
	// sourceParents is m_parents of a class root. Creation adds parents,
	// and merging adds those that remain congruence roots.
	sourceParents                         []*SMTEnode
	disequalities                         *smtEnodeDisequalities // meaningful on class roots
	theoryVariables                       SMTTheoryVarList
	trans                                 smtEqualityEdge
	owner, declaration                    ast.ASTHandle
	labels, propagatedLabels              approx.UintSet
	classSize, generation, functionDeclID uint32
	internalScopeLevel                    uint32
	// The owner keeps its declaration alive for these cached properties.
	family                                ast.FamilyID
	declKind                              int32
	labelHash                             int8
	mark, mark2, interpreted              bool
	suppressArguments, equality           bool
	commutative, boolean, mergeTruthValue bool
	congruenceEnabled, sourceParentMark   bool
	transMark                             bool // transient, one ancestor search
}

// The internalizer has already checked the sort and resolved these children
// without calling a theory. The public constructor accepts a caller's lookup.
type smtTermInputs struct {
	header   ast.ASTNodeHeader
	children []*SMTEnode
}

func NewSMTEnode(m *ast.ASTManager, owner ast.ASTHandle, byApp func(ast.ASTHandle) *SMTEnode, options SMTEnodeOptions) (*SMTEnode, error) {
	return newSMTEnode(m, owner, byApp, options, nil)
}

func newSMTEnode(m *ast.ASTManager, owner ast.ASTHandle, byApp func(ast.ASTHandle) *SMTEnode, options SMTEnodeOptions, term *smtTermInputs) (*SMTEnode, error) {
	if m == nil {
		return nil, ast.ErrASTDomain
	}
	var header ast.ASTNodeHeader
	var boolean bool
	if term != nil {
		header = term.header
	} else {
		var err error
		header, boolean, err = m.ExpressionHeader(owner)
		if err != nil {
			return nil, err
		}
		if header.Kind != ast.ASTApp {
			return nil, ast.ErrASTDomain
		}
	}
	if options.MergeTruthValue && !boolean {
		return nil, ast.ErrASTDomain
	}
	commutative := false
	if !options.SuppressArguments && header.NumArgs == 2 {
		flags, err := m.GetFuncDeclFlags(header.Decl)
		if err != nil {
			return nil, err
		}
		commutative = flags.Commutative
	}
	n := &SMTEnode{
		manager: m, owner: owner, declaration: header.Decl,
		classSize: 1, generation: options.Generation, functionDeclID: math.MaxUint32,
		internalScopeLevel: options.InternalScopeLevel, suppressArguments: options.SuppressArguments,
		equality:    header.Family == m.BasicFamilyID() && header.DeclKind == ast.ASTOpEq,
		commutative: commutative, boolean: boolean, mergeTruthValue: options.MergeTruthValue,
		congruenceEnabled: options.CongruenceEnabled, labelHash: -1,
		theoryVariables: *NewDefaultSMTTheoryVarList(),
		family:          header.Family, declKind: header.DeclKind,
	}
	n.root, n.next = n, n
	if !options.SuppressArguments {
		if term != nil {
			n.arguments = term.children
		} else {
			count := header.NumArgs
			if count != 0 && byApp == nil {
				return nil, ast.ErrASTDomain
			}
			n.arguments = make([]*SMTEnode, count)
			for i := uint32(0); i < count; i++ {
				argument, err := m.GetArg(owner, i)
				if err != nil {
					return nil, err
				}
				child := byApp(argument)
				if child == nil || child.root == nil {
					return nil, ast.ErrASTDomain
				}
				n.arguments[i] = child
			}
			// A lookup can fail or retire an earlier child. Publish the parent
			// links only after every operand and root is available.
			for _, child := range n.arguments {
				if child.root == nil {
					return nil, ast.ErrASTDomain
				}
			}
		}
		if options.UpdateChildrenParent {
			for _, child := range n.arguments {
				child.root.sourceParents = append(child.root.sourceParents, n)
			}
		}
	}
	return n, nil
}

func (n *SMTEnode) valid() error {
	if n == nil || n.manager == nil || n.root == nil || n.next == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (n *SMTEnode) Owner() ast.ASTHandle              { return n.owner }
func (n *SMTEnode) Declaration() ast.ASTHandle        { return n.declaration }
func (n *SMTEnode) Root() *SMTEnode                   { return n.root }
func (n *SMTEnode) Next() *SMTEnode                   { return n.next }
func (n *SMTEnode) ClassSize() uint32                 { return n.classSize }
func (n *SMTEnode) Generation() uint32                { return n.generation }
func (n *SMTEnode) InternalScopeLevel() uint32        { return n.internalScopeLevel }
func (n *SMTEnode) FunctionDeclID() uint32            { return n.functionDeclID }
func (n *SMTEnode) SetFunctionDeclID(id uint32)       { n.functionDeclID = id }
func (n *SMTEnode) IsBool() bool                      { return n.boolean }
func (n *SMTEnode) IsEquality() bool                  { return n.equality }
func (n *SMTEnode) IsMarked() bool                    { return n.mark }
func (n *SMTEnode) IsMarked2() bool                   { return n.mark2 }
func (n *SMTEnode) IsInterpreted() bool               { return n.interpreted }
func (n *SMTEnode) IsCongruenceRoot() bool            { return n.congruence == n }
func (n *SMTEnode) Congruence() *SMTEnode             { return n.congruence }
func (n *SMTEnode) CongruenceEnabled() bool           { return n.congruenceEnabled }
func (n *SMTEnode) IsCommutative() bool               { return n.commutative }
func (n *SMTEnode) MergeTruthValue() bool             { return n.mergeTruthValue }
func (n *SMTEnode) NumArgs() uint32                   { return uint32(len(n.arguments)) }
func (n *SMTEnode) NumParents() uint32                { return uint32(len(n.sourceParents)) }
func (n *SMTEnode) Parents() []*SMTEnode              { return append([]*SMTEnode(nil), n.sourceParents...) }
func (n *SMTEnode) Arguments() []*SMTEnode            { return append([]*SMTEnode(nil), n.arguments...) }
func (n *SMTEnode) Labels() *approx.UintSet           { return &n.labels }
func (n *SMTEnode) PropagatedLabels() *approx.UintSet { return &n.propagatedLabels }

func (n *SMTEnode) Arg(index uint32) (*SMTEnode, error) {
	if err := n.valid(); err != nil {
		return nil, err
	}
	if index >= uint32(len(n.arguments)) {
		return nil, ast.ErrASTDomain
	}
	return n.arguments[index], nil
}

func (n *SMTEnode) OwnerID() (uint32, error)       { return n.manager.GetASTID(n.owner) }
func (n *SMTEnode) DeclarationID() (uint32, error) { return n.manager.GetASTID(n.declaration) }
func (n *SMTEnode) Hash() (uint32, error)          { return n.manager.GetHash(n.owner) }

// smtDisequalityEntry records a disequality's own terms. Their current roots
// give the classes it separates, so merges need not rewrite any entry.
type smtDisequalityEntry struct {
	left, right *SMTEnode
	reason      SMTLiteral
}

// Only roots that receive a disequality need a list header.
type smtEnodeDisequalities struct{ entries []smtDisequalityEntry }

func (n *SMTEnode) disequalityEntries() []smtDisequalityEntry {
	if n.disequalities == nil {
		return nil
	}
	return n.disequalities.entries
}

func (n *SMTEnode) addDisequalities(entries ...smtDisequalityEntry) {
	if len(entries) == 0 {
		return
	}
	if n.disequalities == nil {
		n.disequalities = new(smtEnodeDisequalities)
	}
	n.disequalities.entries = append(n.disequalities.entries, entries...)
}

func (e *smtDisequalityEntry) otherRoot(root *SMTEnode) *SMTEnode {
	if e.left.root == root {
		return e.right.root
	}
	return e.left.root
}

// smtFindDisequality scans the shorter root list, newest first, so the latest
// reason recorded between two classes wins.
func smtFindDisequality(first, second *SMTEnode) (*smtDisequalityEntry, bool) {
	if first == nil || second == nil || first.root == nil || second.root == nil || first.root == second.root {
		return nil, false
	}
	near, far := first.root, second.root
	entries, otherEntries := near.disequalityEntries(), far.disequalityEntries()
	if len(entries) > len(otherEntries) {
		near, far = far, near
		entries = otherEntries
	}
	for i := len(entries) - 1; i >= 0; i-- {
		entry := &entries[i]
		if entry.otherRoot(near) == far {
			return entry, true
		}
	}
	return nil, false
}

// truncateDisequalities ignores lengths above the current one: a scope pop undoes
// merges and disequalities on separate trails, and either can reach a root first.
func (n *SMTEnode) truncateDisequalities(length int) {
	if n.disequalities != nil && length < len(n.disequalities.entries) {
		clear(n.disequalities.entries[length:])
		n.disequalities.entries = n.disequalities.entries[:length]
	}
}

func (n *SMTEnode) removeDisequalitiesWith(other *SMTEnode) {
	if n.disequalities == nil {
		return
	}
	kept := n.disequalities.entries[:0]
	for _, entry := range n.disequalities.entries {
		if entry.otherRoot(n) != other {
			kept = append(kept, entry)
		}
	}
	clear(n.disequalities.entries[len(kept):])
	n.disequalities.entries = kept
}

func (n *SMTEnode) IsTrueEquality() bool {
	return n.equality && len(n.arguments) == 2 && n.arguments[0].root == n.arguments[1].root
}

func (n *SMTEnode) SetMark(value bool) error {
	if err := n.valid(); err != nil || n.mark == value {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	n.mark = value
	return nil
}
func (n *SMTEnode) SetMark2(value bool) error {
	if err := n.valid(); err != nil || n.mark2 == value {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	n.mark2 = value
	return nil
}

func (n *SMTEnode) MarkAsInterpreted() error {
	if err := n.valid(); err != nil {
		return err
	}
	if n.interpreted || len(n.arguments) != 0 || n.classSize != 1 {
		return ast.ErrASTDomain
	}
	n.interpreted = true
	return nil
}

func (n *SMTEnode) NumTheoryVars() uint32 {
	var count uint32
	if n.theoryVariables.variable == SMTNullTheoryVar {
		return 0
	}
	for current := &n.theoryVariables; current != nil; current = current.next {
		count++
	}
	return count
}

func (n *SMTEnode) TheoryVar(theory SMTTheoryID) SMTTheoryVar {
	if n.theoryVariables.variable == SMTNullTheoryVar {
		return SMTNullTheoryVar
	}
	for current := &n.theoryVariables; current != nil; current = current.next {
		if current.theory == theory {
			return current.variable
		}
	}
	return SMTNullTheoryVar
}

func (n *SMTEnode) AddTheoryVar(theory SMTTheoryID, variable SMTTheoryVar) error {
	if err := n.valid(); err != nil {
		return err
	}
	if theory == SMTNullTheoryID || variable == SMTNullTheoryVar || n.TheoryVar(theory) != SMTNullTheoryVar {
		return ast.ErrASTDomain
	}
	if n.theoryVariables.variable == SMTNullTheoryVar {
		n.theoryVariables = SMTTheoryVarList{theory: theory, variable: variable}
		return nil
	}
	current := &n.theoryVariables
	for current.next != nil {
		current = current.next
	}
	current.next = NewSMTTheoryVarList(theory, variable, nil)
	return nil
}

func (n *SMTEnode) ReplaceTheoryVar(theory SMTTheoryID, variable SMTTheoryVar) error {
	if variable == SMTNullTheoryVar {
		return ast.ErrASTDomain
	}
	for current := &n.theoryVariables; current != nil && current.variable != SMTNullTheoryVar; current = current.next {
		if current.theory == theory {
			current.variable = variable
			return nil
		}
	}
	return ast.ErrASTDomain
}

func (n *SMTEnode) DeleteTheoryVar(theory SMTTheoryID) error {
	if n.theoryVariables.variable == SMTNullTheoryVar {
		return ast.ErrASTDomain
	}
	if n.theoryVariables.theory == theory {
		if n.theoryVariables.next == nil {
			n.theoryVariables = *NewDefaultSMTTheoryVarList()
		} else {
			n.theoryVariables = *n.theoryVariables.next
		}
		return nil
	}
	previous := &n.theoryVariables
	for current := previous.next; current != nil; current = current.next {
		if current.theory == theory {
			previous.next = current.next
			return nil
		}
		previous = current
	}
	return ast.ErrASTDomain
}

func (n *SMTEnode) EqualityClassMinGeneration() *SMTEnode {
	best := n
	if n.generation == 0 {
		return best
	}
	for current := n.next; current != n; current = current.next {
		if current.generation < best.generation {
			best = current
			if best.generation == 0 {
				return best
			}
		}
	}
	return best
}

func SMTEnodesCongruent(left, right *SMTEnode) (congruent, commuted bool, err error) {
	if left == nil || right == nil || left.declaration.IsNull() || right.declaration.IsNull() {
		return false, false, ast.ErrASTDomain
	}
	if left.declaration != right.declaration || len(left.arguments) != len(right.arguments) {
		return false, false, nil
	}
	if left.commutative {
		if len(left.arguments) != 2 {
			return false, false, ast.ErrASTDomain
		}
		l0, l1 := left.arguments[0].root, left.arguments[1].root
		r0, r1 := right.arguments[0].root, right.arguments[1].root
		if l0 == r0 && l1 == r1 {
			return true, false, nil
		}
		return l0 == r1 && l1 == r0, l0 == r1 && l1 == r0, nil
	}
	for i := range left.arguments {
		if left.arguments[i].root != right.arguments[i].root {
			return false, false, nil
		}
	}
	return true, false, nil
}

func SMTEnodesMaxGeneration(nodes []*SMTEnode) uint32 {
	var maximum uint32
	for _, node := range nodes {
		if node != nil && node.generation > maximum {
			maximum = node.generation
		}
	}
	return maximum
}

func SMTUnmarkEnodes(nodes []*SMTEnode) error {
	for _, node := range nodes {
		if node == nil || !node.mark {
			return ast.ErrASTDomain
		}
		node.mark = false
	}
	return nil
}

func SMTUnmarkEnodes2(nodes []*SMTEnode) error {
	for _, node := range nodes {
		if node == nil || !node.mark2 {
			return ast.ErrASTDomain
		}
		node.mark2 = false
	}
	return nil
}
