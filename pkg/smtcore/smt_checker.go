package smtcore

// Source: src/smt/smt_checker.h and src/smt/smt_checker.cpp.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// SMTCheckerContext is the concrete read-side context state used by SMTChecker.
// The complete SMT context can own this component and update it as terms are
// internalized, assigned, and merged.
type SMTCheckerContext struct {
	manager       *ast.ASTManager
	relevancy     *SMTRelevancyPropagator
	exprs         *smtExprTable
	relevant      smtRelevantTable
	assignments   smtAssignmentTable
	enodes        smtEnodeTable
	byDeclaration map[ast.ASTHandle][]*SMTEnode
	// decl2enodes is context::m_decl2enodes: per declaration, the enodes
	// with arguments that are not equalities, in creation order.
	// enodes_of(lbl) reads it directly.
	decl2enodes      map[ast.ASTHandle][]*SMTEnode
	disequalityTrail []smtDisequalityUndo
	// diseqTmp is the owner of context::m_is_diseq_tmp: is_diseq builds
	// (= n1 n2) once per argument sort and holds it until the sort changes
	// or the context is flushed.
	diseqTmp     ast.ASTRef
	diseqTmpHeld bool
	// congruence is the owning context's m_cg_table, which
	// get_enode_eq_to(f, args) searches; nil for a standalone view.
	congruence *SMTCongruenceTable
}

// Source context::is_relevant includes all expressions when filtering is
// disabled. Standalone read-side contexts retain their explicitly marked view.
func (c *SMTCheckerContext) isRelevant(expression ast.ASTHandle) bool {
	if c.relevancy != nil {
		return c.relevancy.IsRelevant(expression)
	}
	return c.relevant.get(expression)
}

func NewSMTCheckerContext(manager *ast.ASTManager) (*SMTCheckerContext, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	exprs := newSMTExprTable()
	return &SMTCheckerContext{
		manager: manager, exprs: exprs, relevant: smtRelevantTable{exprs},
		assignments: smtAssignmentTable{t: exprs}, enodes: smtEnodeTable{exprs},
		byDeclaration: make(map[ast.ASTHandle][]*SMTEnode),
		decl2enodes:   make(map[ast.ASTHandle][]*SMTEnode),
	}, nil
}

func (c *SMTCheckerContext) SetRelevant(expression ast.ASTHandle, relevant bool) error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if _, err := c.manager.GetKind(expression); err != nil {
		return err
	}
	c.relevant.set(expression, relevant)
	return nil
}

func (c *SMTCheckerContext) SetAssignment(expression ast.ASTHandle, value lbool.LBool) error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if value != lbool.False && value != lbool.Undef && value != lbool.True {
		return ast.ErrASTDomain
	}
	// Checked builds validate the handle; normal builds, like the AST read
	// accessors, rely on the caller (every hot caller passes a live
	// m_bool_var2expr entry). A null handle is still refused.
	if ast.ASTValidateHandles || smtcoreSourceDebug {
		if _, err := c.manager.GetKind(expression); err != nil {
			return err
		}
	} else if expression.IsNull() {
		return ast.ErrASTHandle
	}
	c.assignments.set(expression, value)
	return nil
}

// bInternalized is context::b_internalized(n).
func (c *SMTCheckerContext) bInternalized(n ast.ASTHandle) bool {
	_, found := c.assignments.lookup(n)
	return found
}

// litInternalized is context::lit_internalized(n).
func (c *SMTCheckerContext) litInternalized(n ast.ASTHandle) bool {
	m := c.manager
	if m.IsFalse(n) {
		return true
	}
	if ok, arg, err := m.IsNot(n); err == nil && ok && c.bInternalized(arg) {
		return true
	}
	return c.bInternalized(n)
}

// getAssignment is context::get_assignment(expr*): false is l_false and a
// not reads its argument. get_assignment_core asserts that the expression
// is internalized; one that is not reads as l_undef here.
func (c *SMTCheckerContext) getAssignment(n ast.ASTHandle) lbool.LBool {
	m := c.manager
	if m.IsFalse(n) {
		return lbool.False
	}
	if ok, arg, err := m.IsNot(n); err == nil && ok {
		return c.assignments.get(arg).Negated()
	}
	return c.assignments.get(n)
}

func (c *SMTCheckerContext) RegisterEnode(node *SMTEnode, relevant bool) error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if node == nil || node.manager != c.manager {
		return ast.ErrASTDomain
	}
	if err := node.valid(); err != nil {
		return err
	}
	if prior := c.enodes.get(node.owner); prior != nil && prior != node {
		return ast.ErrASTDomain
	}
	if c.enodes.get(node.owner) == nil {
		c.enodes.set(node.owner, node)
		c.byDeclaration[node.declaration] = append(c.byDeclaration[node.declaration], node)
		if len(node.arguments) != 0 && !node.IsEquality() {
			c.decl2enodes[node.declaration] = append(c.decl2enodes[node.declaration], node)
		}
	}
	c.relevant.set(node.owner, relevant)
	return nil
}

func (c *SMTCheckerContext) SetDisequal(first, second *SMTEnode, disequal bool) error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if first == nil || second == nil || first.manager != c.manager || second.manager != c.manager || first.root == nil || second.root == nil || first.root == second.root {
		return ast.ErrASTDomain
	}
	r1, r2 := first.root, second.root
	if disequal {
		c.recordDisequality(smtDisequalityEntry{left: first, right: second, reason: SMTNullLiteral})
		return nil
	}
	r1.removeDisequalitiesWith(r2)
	r2.removeDisequalitiesWith(r1)
	return nil
}

// recordDisequality files an entry on both current roots, undone by scope pops.
func (c *SMTCheckerContext) recordDisequality(entry smtDisequalityEntry) {
	r1, r2 := entry.left.root, entry.right.root
	c.disequalityTrail = append(c.disequalityTrail, smtDisequalityUndo{r1, len(r1.disequalities)}, smtDisequalityUndo{r2, len(r2.disequalities)})
	r1.disequalities = append(r1.disequalities, entry)
	r2.disequalities = append(r2.disequalities, entry)
}

// undoDisequalities restores the root lists to a trail length from a scope.
func (c *SMTCheckerContext) undoDisequalities(limit int) error {
	if limit < 0 || limit > len(c.disequalityTrail) {
		return ast.ErrASTDomain
	}
	for len(c.disequalityTrail) > limit {
		index := len(c.disequalityTrail) - 1
		entry := c.disequalityTrail[index]
		entry.root.truncateDisequalities(entry.length)
		c.disequalityTrail[index] = smtDisequalityUndo{}
		c.disequalityTrail = c.disequalityTrail[:index]
	}
	return nil
}

func (c *SMTCheckerContext) isDisequal(first, second *SMTEnode) bool {
	c.holdDiseqTmp(first, second)
	_, found := smtFindDisequality(first, second)
	return found
}

// holdDiseqTmp keeps m_is_diseq_tmp's equality alive as context::is_diseq
// does. It only affects AST lifetimes; a failure leaves the lookup intact.
func (c *SMTCheckerContext) holdDiseqTmp(first, second *SMTEnode) {
	m := c.manager
	if m == nil || first == nil || second == nil {
		return
	}
	if c.diseqTmpHeld {
		current, err := m.RefGet(c.diseqTmp)
		if err != nil {
			return
		}
		arg, err := m.GetArg(current, 0)
		if err != nil {
			return
		}
		held, err1 := m.GetSort(arg)
		wanted, err2 := m.GetSort(first.owner)
		if err1 != nil || err2 != nil || held == wanted {
			return
		}
		// m_manager.dec_ref(m_is_diseq_tmp->get_owner()) before mk_eq.
		c.diseqTmpHeld = false
		if err = m.DelRef(&c.diseqTmp); err != nil {
			return
		}
	}
	eq, err := m.MkEq(first.owner, second.owner)
	if err != nil {
		return
	}
	ref, err := m.NewRef(eq)
	if err != nil {
		return
	}
	c.diseqTmp, c.diseqTmpHeld = ref, true
}

// releaseDiseqTmp is context::flush's release of m_is_diseq_tmp.
func (c *SMTCheckerContext) releaseDiseqTmp() error {
	if c == nil || !c.diseqTmpHeld {
		return nil
	}
	c.diseqTmpHeld = false
	return c.manager.DelRef(&c.diseqTmp)
}

// SMTChecker is smt::checker. Like the source it keeps its caches between
// queries and resets them after each one (m_is_true_cache[2],
// m_to_enode_cache), and caches only expressions whose reference count
// exceeds one.
type SMTChecker struct {
	context *SMTCheckerContext
	// cache holds m_is_true_cache[2] and m_to_enode_cache.
	cache    smtCheckerCache
	bindings []*SMTEnode
	// argBuffer backs get_enode_eq_to_core's ptr_buffer<enode>: each
	// nested call appends above its caller's slice and truncates back.
	argBuffer []*SMTEnode
}

func NewSMTChecker(context *SMTCheckerContext) (*SMTChecker, error) {
	if context == nil || context.manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &SMTChecker{context: context}, nil
}

// clear is the end of is_sat/is_unsat: the caches are reset and the
// flet-scoped bindings restored.
func (c *SMTChecker) clear() {
	c.cache.clear()
	c.bindings = nil
	clear(c.argBuffer)
	c.argBuffer = c.argBuffer[:0]
}

// smtHandleKey is a map key for a node of one manager (slot and
// generation), the role of the source's AST pointer keys: the checker
// caches and the congruence table's per-declaration map only hold
// expressions of the context's manager.
func smtHandleKey(h ast.ASTHandle) uint64 {
	return uint64(h.Slot())<<32 | uint64(h.Generation())
}

func (c *SMTChecker) allArgs(expression ast.ASTHandle, n uint32, value bool) (bool, error) {
	m := c.context.manager
	for i := uint32(0); i < n; i++ {
		argument, err := m.GetArg(expression, i)
		if err != nil {
			return false, err
		}
		result, err := c.check(argument, value)
		if err != nil || !result {
			return result, err
		}
	}
	return true, nil
}

func (c *SMTChecker) anyArg(expression ast.ASTHandle, n uint32, value bool) (bool, error) {
	m := c.context.manager
	for i := uint32(0); i < n; i++ {
		argument, err := m.GetArg(expression, i)
		if err != nil {
			return false, err
		}
		result, err := c.check(argument, value)
		if err != nil || result {
			return result, err
		}
	}
	return false, nil
}

// fixedArgs reads the arguments of a basic operator with a fixed arity
// (not, iff, ite, =) into out; a different arity is a domain error.
func (c *SMTChecker) fixedArgs(expression ast.ASTHandle, n uint32, out []ast.ASTHandle) error {
	m := c.context.manager
	if int(n) != len(out) {
		return ast.ErrASTDomain
	}
	for i := range out {
		arg, err := m.GetArg(expression, uint32(i))
		if err != nil {
			return err
		}
		out[i] = arg
	}
	return nil
}

func (c *SMTChecker) congruentEnode(declaration ast.ASTHandle, arguments []*SMTEnode) (*SMTEnode, error) {
	if table := c.context.congruence; table != nil {
		// context::get_enode_eq_to: the m_cg_table entry, which holds only
		// congruence roots with congruence closure enabled.
		flags, err := c.context.manager.GetFuncDeclFlags(declaration)
		if err != nil {
			return nil, err
		}
		found, err := table.FindApp(declaration, arguments, flags.Commutative)
		if err != nil || found == nil || !c.context.isRelevant(found.owner) {
			return nil, err
		}
		return found, nil
	}
	for _, candidate := range c.context.byDeclaration[declaration] {
		if len(candidate.arguments) != len(arguments) {
			continue
		}
		matches := true
		for i := range arguments {
			if candidate.arguments[i].root != arguments[i].root {
				matches = false
				break
			}
		}
		if !matches && candidate.commutative && len(arguments) == 2 {
			matches = candidate.arguments[0].root == arguments[1].root && candidate.arguments[1].root == arguments[0].root
		}
		if matches && c.context.isRelevant(candidate.owner) {
			return candidate, nil
		}
	}
	return nil, nil
}

func (c *SMTChecker) enodeEqualTo(expression ast.ASTHandle) (*SMTEnode, error) {
	header, err := c.context.manager.NodeHeader(expression)
	if err != nil {
		return nil, err
	}
	if header.Kind == ast.ASTVar {
		index, err := c.context.manager.GetVarIndex(expression)
		if err != nil || int(index) >= len(c.bindings) {
			return nil, err
		}
		return c.bindings[len(c.bindings)-int(index)-1], nil
	}
	if node := c.context.enodes.get(expression); node != nil && c.context.isRelevant(expression) {
		return node, nil
	}
	if header.Kind != ast.ASTApp || header.NumArgs == 0 {
		return nil, nil
	}
	shared := header.RefCount > 1
	if shared {
		if s := c.cache.find(smtHandleKey(expression)); s != nil && s.hasEnode {
			return s.enode, nil
		}
	}
	node, err := c.enodeEqualToCore(expression, header)
	if err == nil && shared {
		s := c.cache.slot(smtHandleKey(expression))
		s.enode, s.hasEnode = node, true
	}
	return node, err
}

// enodeEqualToCore is get_enode_eq_to_core; congruentEnode applies the
// final relevancy test.
func (c *SMTChecker) enodeEqualToCore(expression ast.ASTHandle, header ast.ASTNodeHeader) (*SMTEnode, error) {
	n := header.NumArgs
	base := len(c.argBuffer)
	defer func() {
		clear(c.argBuffer[base:])
		c.argBuffer = c.argBuffer[:base]
	}()
	for i := uint32(0); i < n; i++ {
		argument, err := c.context.manager.GetArg(expression, i)
		if err != nil {
			return nil, err
		}
		node, err := c.enodeEqualTo(argument)
		if err != nil || node == nil {
			return nil, err
		}
		c.argBuffer = append(c.argBuffer, node)
	}
	return c.congruentEnode(header.Decl, c.argBuffer[base:base+int(n)])
}

func (c *SMTChecker) checkCore(expression ast.ASTHandle, header ast.ASTNodeHeader, value bool) (bool, error) {
	manager := c.context.manager
	if smtcoreSourceDebug { // SASSERT(m_manager.is_bool(n))
		sort, err := manager.GetSort(expression)
		if err != nil {
			return false, err
		}
		boolean, err := manager.MkBoolSort()
		if err != nil {
			return false, err
		}
		if sort != boolean {
			return false, ast.ErrASTDomain
		}
	}
	if c.context.bInternalized(expression) && c.context.isRelevant(expression) {
		assignment := c.context.getAssignment(expression)
		return assignment != lbool.Undef && value == (assignment == lbool.True), nil
	}
	if header.Kind != ast.ASTApp {
		return false, nil
	}
	if header.Family == manager.BasicFamilyID() {
		op := header.DeclKind
		var arguments [3]ast.ASTHandle
		switch op {
		case ast.ASTOpTrue:
			return value, nil
		case ast.ASTOpFalse:
			return !value, nil
		case ast.ASTOpNot:
			if err := c.fixedArgs(expression, header.NumArgs, arguments[:1]); err != nil {
				return false, err
			}
			return c.check(arguments[0], !value)
		case ast.ASTOpOr:
			if value {
				return c.anyArg(expression, header.NumArgs, true)
			}
			return c.allArgs(expression, header.NumArgs, false)
		case ast.ASTOpAnd:
			if value {
				return c.allArgs(expression, header.NumArgs, true)
			}
			return c.anyArg(expression, header.NumArgs, false)
		case ast.ASTOpIff:
			if err := c.fixedArgs(expression, header.NumArgs, arguments[:2]); err != nil {
				return false, err
			}
			leftTrue, err := c.check(arguments[0], true)
			if err != nil {
				return false, err
			}
			if leftTrue {
				return c.check(arguments[1], value)
			}
			leftFalse, err := c.check(arguments[0], false)
			if err != nil || !leftFalse {
				return false, err
			}
			return c.check(arguments[1], !value)
		case ast.ASTOpIte:
			if err := c.fixedArgs(expression, header.NumArgs, arguments[:3]); err != nil {
				return false, err
			}
			if c.context.litInternalized(arguments[0]) && c.context.isRelevant(arguments[0]) {
				switch c.context.getAssignment(arguments[0]) {
				case lbool.False:
					return c.check(arguments[2], value)
				case lbool.Undef:
					return false, nil
				case lbool.True:
					return c.check(arguments[1], value)
				}
			}
			thenResult, err := c.check(arguments[1], value)
			if err != nil || !thenResult {
				return false, err
			}
			return c.check(arguments[2], value)
		case ast.ASTOpEq:
			if err := c.fixedArgs(expression, header.NumArgs, arguments[:2]); err != nil {
				return false, err
			}
			left, err := c.enodeEqualTo(arguments[0])
			if err != nil {
				return false, err
			}
			right, err := c.enodeEqualTo(arguments[1])
			if err != nil || left == nil || right == nil || !c.context.isRelevant(left.owner) || !c.context.isRelevant(right.owner) {
				return false, err
			}
			if value {
				return left.root == right.root, nil
			}
			return c.context.isDisequal(left, right), nil
		}
	}
	node, err := c.enodeEqualTo(expression)
	if err != nil || node == nil || !node.boolean || !c.context.isRelevant(node.owner) {
		return false, err
	}
	assignment := c.context.getAssignment(node.owner)
	return assignment != lbool.Undef && value == (assignment == lbool.True), nil
}

func (c *SMTChecker) check(expression ast.ASTHandle, value bool) (bool, error) {
	v := 0
	if value {
		v = 1
	}
	header, err := c.context.manager.NodeHeader(expression)
	if err != nil {
		return false, err
	}
	shared := header.RefCount > 1
	if shared {
		if s := c.cache.find(smtHandleKey(expression)); s != nil && s.truth[v] != 0 {
			return s.truth[v] == 2, nil
		}
	}
	result, err := c.checkCore(expression, header, value)
	if err == nil && shared {
		s := c.cache.slot(smtHandleKey(expression))
		s.truth[v] = 1
		if result {
			s.truth[v] = 2
		}
	}
	return result, err
}

func (c *SMTChecker) run(expression ast.ASTHandle, value bool, bindings []*SMTEnode) (bool, error) {
	if c == nil || c.context == nil || c.context.manager == nil {
		return false, ast.ErrASTHandle
	}
	for _, binding := range bindings {
		if binding == nil || binding.manager != c.context.manager {
			return false, ast.ErrASTDomain
		}
	}
	c.bindings = bindings
	defer c.clear()
	return c.check(expression, value)
}

func (c *SMTChecker) IsSatisfiable(expression ast.ASTHandle, bindings []*SMTEnode) (bool, error) {
	return c.run(expression, true, bindings)
}

func (c *SMTChecker) IsUnsatisfiable(expression ast.ASTHandle, bindings []*SMTEnode) (bool, error) {
	return c.run(expression, false, bindings)
}
