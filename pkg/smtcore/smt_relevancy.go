package smtcore

// Source: src/smt/smt_relevancy.h and src/smt/smt_relevancy.cpp.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtRelevancyHandlerKind uint8

const (
	smtRelevancySimple smtRelevancyHandlerKind = iota
	smtRelevancyPair
	smtRelevancyAnd
	smtRelevancyOr
	smtRelevancyITE
	smtRelevancyTermITE
	smtRelevancyPairTargets
	smtRelevancyAssertedOr
)

type smtRelevancyHandler struct {
	target     ast.ASTHandle
	pair       *smtRelevancyPairData
	assertedOr *smtAssertedOr
	kind       smtRelevancyHandlerKind
	// pooled is 1 + the handler's position in the propagator's ehs once a
	// table holds it (the relevancy_eh object a watch list points to).
	pooled uint32
}

// Pair dependencies store their two prerequisites here. A term ITE stores
// the equality to mark for its then and else branches in the same order.
type smtRelevancyPairData struct {
	first, second ast.ASTHandle
	targets       []ast.ASTHandle
}

type smtRelevancyTrailKind uint8

const (
	smtRelevancyHandlerTrail smtRelevancyTrailKind = iota
	smtRelevancyPositiveWatchTrail
	smtRelevancyNegativeWatchTrail
	smtRelevancyRefTrail // an owned reference without a list link
)

const smtRelevancyWatchOwnsParent smtRelevancyTrailKind = 1 << 7

type smtRelevancyTrail struct {
	kind       smtRelevancyTrailKind
	expression ast.ASTHandle
}

type smtRelevancyScope struct {
	relevantLimit int
	trailLimit    int
	ehLimit       int
}

type SMTRelevancyPropagator struct {
	context       *SMTCheckerContext
	internalizer  *SMTInternalizerState // borrowed logical-context callback owner
	enabled       bool
	relevant      []ast.ASTHandle
	qhead         int
	handlers      smtRelevancyTable
	positiveWatch smtRelevancyTable
	negativeWatch smtRelevancyTable
	trail         []smtRelevancyTrail
	// ehs holds the handlers the tables refer to, as z3 allocates each
	// relevancy_eh once in the context region and its watch lists hold
	// pointers; a scope's handlers die with it (region pop).
	ehs                   []smtRelevancyHandler
	scopes                []smtRelevancyScope
	newlyRelevant         []ast.ASTHandle
	newlyRelevantRemovals uint64
	// After the last filter or drain, setRelevant appended the same handles
	// to the notification queue and the owning relevant trail, in order.
	newlyRelevantBase int
	propagating       bool
	// An interrupted pop has already unmarked the expressions of this scope.
	// Zero means idle; otherwise retry at scopes[interruptedPop-1].
	interruptedPop int
}

func NewSMTRelevancyPropagator(context *SMTCheckerContext, enabled bool) (*SMTRelevancyPropagator, error) {
	if context == nil || context.manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &SMTRelevancyPropagator{
		context: context, enabled: enabled,
	}, nil
}

func (p *SMTRelevancyPropagator) IsRelevant(expression ast.ASTHandle) bool {
	return p != nil && (!p.enabled || p.context.relevant.get(expression))
}

func (p *SMTRelevancyPropagator) assignment(expression ast.ASTHandle) lbool.LBool {
	if s := p.internalizer; s != nil {
		if variable, found := s.boolVar.lookup(expression); found {
			if child := s.notChild(variable); !child.IsNull() {
				return p.directAssignment(child).Negated()
			}
			return s.assignment[NewSMTLiteral(variable, false).Index()]
		}
	}
	// Source find_assignment reads the atom beneath a NOT. A nongate NOT
	// can also own a Boolean variable; assignment events use its direct value.
	if child, err := p.negatedArgument(expression); err == nil && !child.IsNull() {
		return p.directAssignment(child).Negated()
	}
	return p.directAssignment(expression)
}

// Neither reserved truth literal has an ordinary Boolean variable binding.
// A NOT operand is read directly, even when that operand is another NOT.
func (p *SMTRelevancyPropagator) directAssignment(expression ast.ASTHandle) lbool.LBool {
	switch {
	case p.context.manager.IsTrue(expression):
		return lbool.True
	case p.context.manager.IsFalse(expression):
		return lbool.False
	default:
		return p.context.assignments.get(expression)
	}
}

func (p *SMTRelevancyPropagator) negatedArgument(expression ast.ASTHandle) (ast.ASTHandle, error) {
	// The source inspects the node once. IsNot combines kind, family,
	// declaration and child lookup under one manager lock in the Go arena.
	_, child, err := p.context.manager.IsNot(expression)
	return child, err
}

// Source context::get_literal strips one NOT. Its operand can have its own
// Boolean variable, whose assignment event is what add_rel_watch watches.
func (p *SMTRelevancyPropagator) literalWatch(expression ast.ASTHandle, value bool) (ast.ASTHandle, bool, error) {
	child, err := p.negatedArgument(expression)
	if err != nil || child.IsNull() {
		return expression, value, err
	}
	return child, !value, nil
}

func (p *SMTRelevancyPropagator) addLiteralWatch(expression ast.ASTHandle, value bool, handler *smtRelevancyHandler) error {
	expression, value, err := p.literalWatch(expression, value)
	if err != nil {
		return err
	}
	return p.addWatch(expression, value, handler)
}

// setRelevant is set_relevant: mark_as_relevant calls it for every member
// of n's class without checking whether the member is already relevant, so
// a member can be recorded (and relevant_eh run) twice.
func (p *SMTRelevancyPropagator) setRelevant(expression ast.ASTHandle) error {
	// A refused retain must leave the mark and its notification retryable.
	if err := p.context.manager.IncRef(expression); err != nil {
		return err
	}
	if p.context.relevant.set(expression, true) {
		// Popping a second mark can invalidate an earlier notification for
		// the same expression. Recheck the existing queue in that case.
		p.newlyRelevantBase = len(p.newlyRelevant)
	}
	p.relevant = append(p.relevant, expression)
	p.newlyRelevant = append(p.newlyRelevant, expression)
	if p.internalizer != nil {
		return p.internalizer.notifyRelevant(expression)
	}
	return nil
}

func (p *SMTRelevancyPropagator) MarkRelevant(expression ast.ASTHandle) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if p.interruptedPop != 0 {
		return ast.ErrASTPending
	}
	if !p.enabled || p.context.relevant.get(expression) {
		return nil
	}
	if err := p.context.manager.ValidateHandle(expression); err != nil {
		return err
	}
	if node := p.context.enodes.get(expression); node != nil {
		start := node
		for current := start; ; current = current.next {
			if current == nil || current.owner.IsNull() {
				return ast.ErrASTDomain
			}
			if err := p.setRelevant(current.owner); err != nil {
				return err
			}
			if current.next == start {
				break
			}
		}
	} else {
		if err := p.setRelevant(expression); err != nil {
			return err
		}
	}
	return nil
}

func (p *SMTRelevancyPropagator) DrainNewlyRelevant() []ast.ASTHandle {
	if p == nil || p.interruptedPop != 0 {
		return nil
	}
	result := append([]ast.ASTHandle(nil), p.newlyRelevant...)
	p.newlyRelevant = nil
	p.newlyRelevantBase = 0
	return result
}

// Theory propagation has already delivered these marks through notifyRelevant.
// Keep up to one short batch for the next propagation round.
func (p *SMTRelevancyPropagator) discardNewlyRelevant() {
	p.newlyRelevantBase = 0
	const maxRetainedMarks = 512 // 8 KiB of pointer-free handles.
	if cap(p.newlyRelevant) > maxRetainedMarks {
		p.newlyRelevant = nil
	} else {
		p.newlyRelevant = p.newlyRelevant[:0]
	}
}

// pool returns handler's ehs reference, storing it on first use.
func (p *SMTRelevancyPropagator) pool(handler *smtRelevancyHandler) uint32 {
	if handler.pooled == 0 {
		p.ehs = append(p.ehs, *handler)
		handler.pooled = uint32(len(p.ehs))
		p.ehs[len(p.ehs)-1].pooled = handler.pooled
	}
	return handler.pooled
}

// invokeRef runs a stored handler. The handler is copied first: running
// it can store new handlers and move ehs.
func (p *SMTRelevancyPropagator) invokeRef(ref uint32) error {
	handler := p.ehs[ref-1]
	return p.invoke(&handler)
}

func (p *SMTRelevancyPropagator) invoke(handler *smtRelevancyHandler) error {
	switch handler.kind {
	case smtRelevancySimple:
		return p.MarkRelevant(handler.target)
	case smtRelevancyPair:
		if p.IsRelevant(handler.pair.first) && p.IsRelevant(handler.pair.second) {
			return p.MarkRelevant(handler.target)
		}
		return nil
	case smtRelevancyPairTargets:
		if p.IsRelevant(handler.pair.first) && p.IsRelevant(handler.pair.second) {
			for _, target := range handler.pair.targets {
				if err := p.MarkRelevant(target); err != nil {
					return err
				}
			}
		}
		return nil
	case smtRelevancyAnd:
		if p.IsRelevant(handler.target) {
			return p.propagateAnd(handler.target)
		}
		return nil
	case smtRelevancyOr:
		if p.IsRelevant(handler.target) {
			return p.propagateOr(handler.target)
		}
		return nil
	case smtRelevancyAssertedOr:
		return p.propagateAssertedOr(handler.target, handler.assertedOr)
	case smtRelevancyITE:
		if p.IsRelevant(handler.target) {
			return p.propagateITE(handler.target)
		}
		return nil
	case smtRelevancyTermITE:
		if !p.IsRelevant(handler.target) {
			return nil
		}
		condition, err := p.context.manager.GetArg(handler.target, 0)
		if err != nil {
			return err
		}
		if err = p.MarkRelevant(condition); err != nil {
			return err
		}
		switch p.assignment(condition) {
		case lbool.False:
			return p.MarkRelevant(handler.pair.second)
		case lbool.True:
			return p.MarkRelevant(handler.pair.first)
		}
		return nil
	default:
		return ast.ErrASTDomain
	}
}

func (p *SMTRelevancyPropagator) addHandler(source ast.ASTHandle, handler *smtRelevancyHandler) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if p.interruptedPop != 0 {
		return ast.ErrASTPending
	}
	if !p.enabled {
		return nil
	}
	if p.context.relevant.get(source) {
		return p.invoke(handler)
	}
	return p.register(&p.handlers, smtRelevancyHandlerTrail, source, handler)
}

func (p *SMTRelevancyPropagator) AddDependency(source, target ast.ASTHandle) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	return p.addHandler(source, &smtRelevancyHandler{kind: smtRelevancySimple, target: target})
}

func (p *SMTRelevancyPropagator) AddPairDependency(first, second, target ast.ASTHandle) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	handler := smtRelevancyHandler{kind: smtRelevancyPair, target: target, pair: &smtRelevancyPairData{first: first, second: second}}
	return p.addPairHandler(&handler)
}

// AddPairTargetsDependency installs one handler, as arith_eq_relevancy_eh
// does, that marks every target in order once first and second are both
// relevant. Separate per-target handlers would run newest first.
func (p *SMTRelevancyPropagator) AddPairTargetsDependency(first, second ast.ASTHandle, targets ...ast.ASTHandle) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	pair := &smtRelevancyPairData{first: first, second: second, targets: append([]ast.ASTHandle(nil), targets...)}
	handler := smtRelevancyHandler{kind: smtRelevancyPairTargets, pair: pair}
	return p.addPairHandler(&handler)
}

func (p *SMTRelevancyPropagator) addPairHandler(handler *smtRelevancyHandler) error {
	if p.interruptedPop != 0 {
		return ast.ErrASTPending
	}
	if !p.enabled {
		return nil
	}
	first, second := handler.pair.first, handler.pair.second
	if p.IsRelevant(first) || p.IsRelevant(second) {
		// An already-relevant prerequisite invokes the handler immediately.
		if err := p.addHandler(first, handler); err != nil {
			return err
		}
		return p.addHandler(second, handler)
	}
	if err := p.context.manager.IncRefPair(first, second); err != nil {
		return err
	}
	p.linkRetained(&p.handlers, smtRelevancyHandlerTrail, first, handler)
	p.linkRetained(&p.handlers, smtRelevancyHandlerTrail, second, handler)
	return nil
}

func (p *SMTRelevancyPropagator) AddAndHandler(parent ast.ASTHandle) error {
	return p.addHandler(parent, &smtRelevancyHandler{kind: smtRelevancyAnd, target: parent})
}
func (p *SMTRelevancyPropagator) AddOrHandler(parent ast.ASTHandle) error {
	return p.addHandler(parent, &smtRelevancyHandler{kind: smtRelevancyOr, target: parent})
}
func (p *SMTRelevancyPropagator) AddITEHandler(parent ast.ASTHandle) error {
	return p.addHandler(parent, &smtRelevancyHandler{kind: smtRelevancyITE, target: parent})
}
func (p *SMTRelevancyPropagator) AddTermITEHandler(parent, thenEquality, elseEquality ast.ASTHandle) error {
	pair := &smtRelevancyPairData{first: thenEquality, second: elseEquality}
	return p.addHandler(parent, &smtRelevancyHandler{kind: smtRelevancyTermITE, target: parent, pair: pair})
}

func (p *SMTRelevancyPropagator) addWatch(expression ast.ASTHandle, value bool, handler *smtRelevancyHandler) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if p.interruptedPop != 0 {
		return ast.ErrASTPending
	}
	if !p.enabled {
		return nil
	}
	assignment := p.assignment(expression)
	if !value {
		assignment = assignment.Negated()
	}
	return p.addWatchForAssignment(expression, value, handler, assignment)
}

func (p *SMTRelevancyPropagator) addWatchForAssignment(expression ast.ASTHandle, value bool, handler *smtRelevancyHandler, assignment lbool.LBool) error {
	switch assignment {
	case lbool.False:
		return nil
	case lbool.True:
		return p.invoke(handler)
	case lbool.Undef:
		if value {
			return p.register(&p.positiveWatch, smtRelevancyPositiveWatchTrail, expression, handler)
		}
		return p.register(&p.negativeWatch, smtRelevancyNegativeWatchTrail, expression, handler)
	default:
		return ast.ErrASTDomain
	}
}

func (p *SMTRelevancyPropagator) AddWatchTarget(expression ast.ASTHandle, value bool, target ast.ASTHandle) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if !p.enabled {
		return nil
	}
	if p.interruptedPop != 0 {
		return ast.ErrASTPending
	}
	// The assignment read may inspect a NOT's operand before a watch is
	// retained, including when a satisfied watch fires immediately.
	if err := p.context.manager.ValidateHandle(expression); err != nil {
		return err
	}
	assignment := p.assignment(expression)
	if !value {
		assignment = assignment.Negated()
	}
	if assignment == lbool.True {
		if err := p.MarkRelevant(target); err != nil {
			return err
		}
		return p.Propagate()
	}
	return p.addWatch(expression, value, &smtRelevancyHandler{kind: smtRelevancySimple, target: target})
}

// AddAndWatches installs the shared handler used by the internalizer for a
// Boolean conjunction. A false child can select itself once parent is relevant.
func (p *SMTRelevancyPropagator) AddAndWatches(parent ast.ASTHandle) error {
	handler := smtRelevancyHandler{kind: smtRelevancyAnd, target: parent}
	count, err := p.validateBasicApplication(parent, ast.ASTOpAnd)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		child, err := p.context.manager.GetArg(parent, index)
		if err != nil {
			return err
		}
		child, value, err := p.literalWatch(child, false)
		if err != nil {
			return err
		}
		if err = p.addWatch(child, value, &handler); err != nil {
			return err
		}
	}
	return nil
}

// AddOrWatches installs the shared handler used by the internalizer for a
// Boolean disjunction. A true child can select itself once parent is relevant.
func (p *SMTRelevancyPropagator) AddOrWatches(parent ast.ASTHandle) error {
	return p.addOrWatches(parent, smtRelevancyOr)
}

func (p *SMTRelevancyPropagator) addOrWatches(parent ast.ASTHandle, kind smtRelevancyHandlerKind) error {
	handler := smtRelevancyHandler{kind: kind, target: parent}
	count, err := p.validateBasicApplication(parent, ast.ASTOpOr)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		child, err := p.context.manager.GetArg(parent, index)
		if err != nil {
			return err
		}
		child, value, err := p.literalWatch(child, true)
		if err != nil {
			return err
		}
		if err = p.addWatch(child, value, &handler); err != nil {
			return err
		}
	}
	return nil
}

// AddITEWatches installs both condition watches used for a Boolean ITE.
func (p *SMTRelevancyPropagator) AddITEWatches(parent ast.ASTHandle) error {
	if _, err := p.validateBasicApplication(parent, ast.ASTOpIte); err != nil {
		return err
	}
	condition, err := p.context.manager.GetArg(parent, 0)
	if err != nil {
		return err
	}
	handler := smtRelevancyHandler{kind: smtRelevancyITE, target: parent}
	if err = p.addLiteralWatch(condition, true, &handler); err != nil {
		return err
	}
	return p.addLiteralWatch(condition, false, &handler)
}

// AddTermITEWatches installs the parent handler and both condition watches
// used by internalize_ite_term.
func (p *SMTRelevancyPropagator) AddTermITEWatches(parent, thenEquality, elseEquality ast.ASTHandle) error {
	if _, err := p.validateBasicApplication(parent, ast.ASTOpIte); err != nil {
		return err
	}
	condition, err := p.context.manager.GetArg(parent, 0)
	if err != nil {
		return err
	}
	pair := &smtRelevancyPairData{first: thenEquality, second: elseEquality}
	handler := smtRelevancyHandler{kind: smtRelevancyTermITE, target: parent, pair: pair}
	if err = p.addLiteralWatch(condition, true, &handler); err != nil {
		return err
	}
	if err = p.addLiteralWatch(condition, false, &handler); err != nil {
		return err
	}
	return p.addHandler(parent, &handler)
}

func (p *SMTRelevancyPropagator) validateBasicApplication(expression ast.ASTHandle, operation int32) (uint32, error) {
	if p == nil || p.context == nil {
		return 0, ast.ErrASTHandle
	}
	// The watches retain the children, so validate the supplied parent
	// before reading its slot to choose those children.
	if err := p.context.manager.ValidateHandle(expression); err != nil {
		return 0, err
	}
	kind, err := p.context.manager.GetKind(expression)
	if err != nil {
		return 0, err
	}
	if kind != ast.ASTApp {
		return 0, ast.ErrASTDomain
	}
	family, err := p.context.manager.GetFamilyID(expression)
	if err != nil {
		return 0, err
	}
	op, err := p.context.manager.GetDeclKind(expression)
	if err != nil {
		return 0, err
	}
	if family != p.context.manager.BasicFamilyID() || op != operation {
		return 0, ast.ErrASTDomain
	}
	return p.context.manager.GetNumArgs(expression)
}

func (p *SMTRelevancyPropagator) propagateAll(expression ast.ASTHandle) error {
	arguments, _, err := p.gateArguments(expression)
	if err != nil {
		return err
	}
	for index := arguments.Count(); index > 0; index-- {
		argument, err := arguments.At(index - 1)
		if err != nil {
			return err
		}
		if err = p.MarkRelevant(argument); err != nil {
			return err
		}
	}
	return nil
}

// gateArguments is the application's argument array (to_app(n)->get_args()):
// the internalizer's recorded gate arguments, or a view of the
// application's own; nothing is copied.
func (p *SMTRelevancyPropagator) gateArguments(expression ast.ASTHandle) (ast.AppArguments, []SMTLiteral, error) {
	if s := p.internalizer; s != nil {
		if variable, found := s.boolVar.lookup(expression); found {
			if gate := s.boolGate(variable); gate != nil {
				return ast.SliceArguments(gate.args), gate.literals, nil
			}
		}
	}
	view, err := p.context.manager.AppArgumentView(expression)
	return view, nil, err
}

func (p *SMTRelevancyPropagator) propagateOr(expression ast.ASTHandle) error {
	value := p.assignment(expression)
	if value == lbool.Undef {
		value = lbool.True
	}
	if value == lbool.False {
		return p.propagateAll(expression)
	}
	if value != lbool.True {
		return nil
	}
	arguments, literals, err := p.gateArguments(expression)
	if err != nil {
		return err
	}
	var selected ast.ASTHandle
	for index := uint32(0); index < arguments.Count(); index++ {
		argument, err := arguments.At(index)
		if err != nil {
			return err
		}
		var value lbool.LBool
		if literals != nil {
			value = p.internalizer.assignment[literals[index].Index()]
		} else {
			value = p.assignment(argument)
		}
		if value == lbool.True {
			if p.context.relevant.get(argument) {
				return nil
			}
			if selected.IsNull() {
				selected = argument
			}
		}
	}
	if !selected.IsNull() {
		return p.MarkRelevant(selected)
	}
	return nil
}

func (p *SMTRelevancyPropagator) propagateAnd(expression ast.ASTHandle) error {
	value := p.assignment(expression)
	if value == lbool.True {
		return p.propagateAll(expression)
	}
	if value != lbool.False {
		return nil
	}
	arguments, literals, err := p.gateArguments(expression)
	if err != nil {
		return err
	}
	var selected ast.ASTHandle
	for index := uint32(0); index < arguments.Count(); index++ {
		argument, err := arguments.At(index)
		if err != nil {
			return err
		}
		var value lbool.LBool
		if literals != nil {
			value = p.internalizer.assignment[literals[index].Index()]
		} else {
			value = p.assignment(argument)
		}
		if value == lbool.False {
			if p.context.relevant.get(argument) {
				return nil
			}
			if selected.IsNull() {
				selected = argument
			}
		}
	}
	if !selected.IsNull() {
		return p.MarkRelevant(selected)
	}
	return nil
}

func (p *SMTRelevancyPropagator) propagateITE(expression ast.ASTHandle) error {
	condition, err := p.context.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	if err = p.MarkRelevant(condition); err != nil {
		return err
	}
	index := uint32(0)
	switch p.assignment(condition) {
	case lbool.False:
		index = 2
	case lbool.True:
		index = 1
	default:
		return nil
	}
	branch, err := p.context.manager.GetArg(expression, index)
	if err != nil {
		return err
	}
	return p.MarkRelevant(branch)
}

func (p *SMTRelevancyPropagator) Propagate() error {
	return p.propagateWithOr(ast.ASTHandle{}, nil)
}

// The just-asserted OR already has a watch record with its signed child
// bindings. Reuse it when the ordinary queue reaches that exact parent.
func (p *SMTRelevancyPropagator) propagateWithOr(parent ast.ASTHandle, gate *smtAssertedOr) (result error) {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if p.interruptedPop != 0 {
		return ast.ErrASTPending
	}
	if !p.enabled || p.propagating {
		return nil
	}
	p.propagating = true
	var head int
	var expression ast.ASTHandle
	defer func() {
		p.propagating = false
		// Retry the failed expression if a callback left it in the queue.
		// Completed marks are already relevant and won't be delivered again.
		if result != nil && p.qhead == head+1 && head < len(p.relevant) && p.relevant[head] == expression {
			p.qhead = head
		}
	}()
	for p.qhead < len(p.relevant) {
		head = p.qhead
		expression = p.relevant[head]
		p.tracePropagate(expression)
		p.qhead++
		kind, err := p.context.manager.GetKind(expression)
		if err != nil {
			return err
		}
		if kind == ast.ASTApp {
			family, err := p.context.manager.GetFamilyID(expression)
			if err != nil {
				return err
			}
			if family == p.context.manager.BasicFamilyID() {
				op, err := p.context.manager.GetDeclKind(expression)
				if err != nil {
					return err
				}
				switch op {
				case ast.ASTOpOr:
					if gate != nil && expression == parent {
						err = p.propagateOrWithBindings(parent, gate, p.context.exprs.read(parent))
					} else {
						err = p.propagateOr(expression)
					}
				case ast.ASTOpAnd:
					err = p.propagateAnd(expression)
				case ast.ASTOpIte:
					err = p.propagateITE(expression)
				default:
					err = p.propagateAll(expression)
				}
			} else {
				err = p.propagateAll(expression)
			}
			if err != nil {
				return err
			}
		}
		if err := p.invokeList(&p.handlers, expression); err != nil {
			return err
		}
	}
	return nil
}

func (p *SMTRelevancyPropagator) CanPropagate() bool {
	return p != nil && p.interruptedPop == 0 && p.qhead < len(p.relevant)
}

func (p *SMTRelevancyPropagator) RelevantExpressions() []ast.ASTHandle {
	if p == nil || !p.enabled {
		return nil
	}
	expressions := p.relevant
	if p.interruptedPop != 0 {
		expressions = expressions[:p.scopes[p.interruptedPop-1].relevantLimit]
	}
	return append([]ast.ASTHandle(nil), expressions...)
}

func (p *SMTRelevancyPropagator) checkAllArguments(expression ast.ASTHandle) error {
	count, err := p.context.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		argument, err := p.context.manager.GetArg(expression, index)
		if err != nil {
			return err
		}
		if !p.IsRelevant(argument) {
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (p *SMTRelevancyPropagator) checkOr(expression ast.ASTHandle) error {
	if p.assignment(expression) == lbool.False {
		return p.checkAllArguments(expression)
	}
	count, err := p.context.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		argument, err := p.context.manager.GetArg(expression, index)
		if err != nil {
			return err
		}
		if p.assignment(argument) == lbool.True && p.IsRelevant(argument) {
			return nil
		}
	}
	return ast.ErrASTDomain
}

func (p *SMTRelevancyPropagator) checkAnd(expression ast.ASTHandle) error {
	if p.assignment(expression) == lbool.True {
		return p.checkAllArguments(expression)
	}
	if p.assignment(expression) != lbool.False {
		return nil
	}
	count, err := p.context.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		argument, err := p.context.manager.GetArg(expression, index)
		if err != nil {
			return err
		}
		if p.assignment(argument) == lbool.False && p.IsRelevant(argument) {
			return nil
		}
	}
	return ast.ErrASTDomain
}

func (p *SMTRelevancyPropagator) checkITE(expression ast.ASTHandle) error {
	condition, err := p.context.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	if !p.IsRelevant(condition) {
		return ast.ErrASTDomain
	}
	var branchIndex uint32
	switch p.assignment(condition) {
	case lbool.False:
		branchIndex = 2
	case lbool.True:
		branchIndex = 1
	default:
		return nil
	}
	branch, err := p.context.manager.GetArg(expression, branchIndex)
	if err != nil {
		return err
	}
	sort, err := p.context.manager.GetSort(expression)
	if err != nil {
		return err
	}
	boolean, err := p.context.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if sort == boolean {
		if !p.IsRelevant(branch) {
			return ast.ErrASTDomain
		}
		return nil
	}
	// The internalizer marks the theory's canonical equality for the branch.
	// Standalone term ITE handlers use the raw equality.
	var equality ast.ASTHandle
	if p.internalizer != nil {
		equality, err = p.internalizer.mkEqualityAtom(expression, branch)
	} else {
		equality, err = p.context.manager.MkEq(expression, branch)
	}
	if err != nil {
		return err
	}
	if !p.IsRelevant(equality) {
		return ast.ErrASTDomain
	}
	return nil
}

// Check verifies the source debug invariants for relevant expressions after
// propagation reaches a fixed point.
func (p *SMTRelevancyPropagator) Check(expressions []ast.ASTHandle) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if !p.enabled {
		return nil
	}
	if p.CanPropagate() {
		return ast.ErrASTDomain
	}
	for _, expression := range expressions {
		if !p.IsRelevant(expression) {
			continue
		}
		kind, err := p.context.manager.GetKind(expression)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			if node := p.context.enodes.get(expression); node != nil {
				for current := node.next; current != node; current = current.next {
					if current == nil || !p.IsRelevant(current.owner) {
						return ast.ErrASTDomain
					}
				}
			}
			continue
		}
		family, err := p.context.manager.GetFamilyID(expression)
		if err != nil {
			return err
		}
		if family != p.context.manager.BasicFamilyID() {
			if err = p.checkAllArguments(expression); err != nil {
				return err
			}
			continue
		}
		op, err := p.context.manager.GetDeclKind(expression)
		if err != nil {
			return err
		}
		switch op {
		case ast.ASTOpOr:
			err = p.checkOr(expression)
		case ast.ASTOpAnd:
			err = p.checkAnd(expression)
		case ast.ASTOpIte:
			err = p.checkITE(expression)
		default:
			err = p.checkAllArguments(expression)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *SMTRelevancyPropagator) AssignmentEvent(expression ast.ASTHandle, value bool) error {
	return p.assignmentEvent(expression, value, SMTNullBoolVar)
}

func (p *SMTRelevancyPropagator) assignmentVariableEvent(variable int32, value bool) error {
	return p.assignmentEvent(p.internalizer.boolExpr[variable], value, variable)
}

func (p *SMTRelevancyPropagator) assignmentEvent(expression ast.ASTHandle, value bool, variable int32) error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if p.interruptedPop != 0 {
		return ast.ErrASTPending
	}
	if !p.enabled {
		return nil
	}
	want := lbool.FromBool(value)
	// context::propagate_relevancy passes bool_var2expr and the assigned
	// literal's value, not find_assignment's normalized NOT-child value.
	var assignment lbool.LBool
	direct := false
	if variable != SMTNullBoolVar {
		assignment, direct = p.internalizer.assignment[NewSMTLiteral(variable, false).Index()], true
	} else {
		assignment, direct = p.context.assignments.lookup(expression)
	}
	if !direct {
		assignment = p.assignment(expression)
	}
	if assignment != want {
		return ast.ErrASTDomain
	}
	op := int32(-1)
	if variable != SMTNullBoolVar {
		op = p.internalizer.boolGateOp(variable)
	} else if p.context.relevant.get(expression) {
		family, err := p.context.manager.GetFamilyID(expression)
		if err == nil && family == p.context.manager.BasicFamilyID() {
			op, err = p.context.manager.GetDeclKind(expression)
			if err != nil {
				return err
			}
		}
	}
	if (op == ast.ASTOpOr || op == ast.ASTOpAnd) && p.context.relevant.get(expression) {
		if op == ast.ASTOpOr {
			if err := p.propagateOr(expression); err != nil {
				return err
			}
		} else if op == ast.ASTOpAnd {
			if err := p.propagateAnd(expression); err != nil {
				return err
			}
		}
	}
	if value {
		return p.invokeList(&p.positiveWatch, expression)
	}
	return p.invokeList(&p.negativeWatch, expression)
}

// invokeList runs expression's handlers newest first, walking the list as
// it stood when the walk began (handlers the walk registers are prepended
// and not visited), as z3 walks its relevancy_ehs cons list.
func (p *SMTRelevancyPropagator) invokeList(table *smtRelevancyTable, expression ast.ASTHandle) error {
	for index := table.head(expression); index != 0; {
		cell := table.link(index)
		index = cell.previous
		if err := p.invokeRef(cell.handler); err != nil {
			return err
		}
	}
	return nil
}

func (p *SMTRelevancyPropagator) PushScope() error {
	if p == nil || p.context == nil {
		return ast.ErrASTHandle
	}
	if p.interruptedPop != 0 {
		return ast.ErrASTPending
	}
	p.scopes = append(p.scopes, smtRelevancyScope{relevantLimit: len(p.relevant), trailLimit: len(p.trail), ehLimit: len(p.ehs)})
	return nil
}

func (p *SMTRelevancyPropagator) PopScopes(count uint32) error {
	if p == nil || p.context == nil || count == 0 || count > uint32(len(p.scopes)) {
		return ast.ErrASTDomain
	}
	target := len(p.scopes) - int(count)
	for len(p.scopes) > target {
		level := target
		additions := p.context.exprs.relevanceAdditions
		retiredNotifications := 0
		uniqueSuffix := false
		if p.interruptedPop != 0 {
			level = p.interruptedPop - 1
			if target > level {
				return ast.ErrASTPending
			}
		} else {
			uniqueSuffix = p.newlyRelevantBase == 0 && p.context.exprs.relevanceRemovals == p.newlyRelevantRemovals
			retiredNotifications = min(len(p.newlyRelevant)-p.newlyRelevantBase, len(p.relevant)-p.scopes[level].relevantLimit)
			for i := len(p.relevant); i > p.scopes[level].relevantLimit; i-- {
				h := p.relevant[i-1]
				p.context.relevant.del(h)
				p.traceUnmark(h)
			}
		}
		if err := p.finishPop(level, retiredNotifications, additions, uniqueSuffix); err != nil {
			p.interruptedPop = level + 1
			return err
		}
		p.interruptedPop = 0
	}
	return nil
}

func (p *SMTRelevancyPropagator) finishPop(newLevel, retiredNotifications int, additions uint64, uniqueSuffix bool) error {
	scope := p.scopes[newLevel]
	var err error
	p.relevant, err = p.context.manager.ShrinkHeldRefs(p.relevant, scope.relevantLimit)
	if err != nil {
		return err
	}
	p.qhead = len(p.relevant)
	// A pop that removed no relevance marks cannot invalidate queued work.
	// Count explicit mark removals too, since the checker can update them.
	removals := p.context.exprs.relevanceRemovals
	if removals != p.newlyRelevantRemovals {
		checkSurvivors := true
		// A callback during unmarking or reference release can drain the
		// queue or reactivate a mark. An interrupted pop also rechecks
		// the entire queue on retry.
		if retiredNotifications <= len(p.newlyRelevant) && additions == p.context.exprs.relevanceAdditions {
			p.newlyRelevant = p.newlyRelevant[:len(p.newlyRelevant)-retiredNotifications]
			// Each retired queue entry accounts for one mark removal. If the
			// entire queue was unique, its surviving entries are still marked.
			checkSurvivors = !uniqueSuffix || removals-p.newlyRelevantRemovals != uint64(retiredNotifications)
		}
		if checkSurvivors {
			kept := p.newlyRelevant[:0]
			for _, expression := range p.newlyRelevant {
				if p.context.relevant.get(expression) {
					kept = append(kept, expression)
				}
			}
			p.newlyRelevant = kept
			p.newlyRelevantBase = len(kept)
		}
		p.newlyRelevantRemovals = removals
	} else {
		p.newlyRelevantBase = len(p.newlyRelevant)
	}
	if err := p.undoTrail(scope.trailLimit); err != nil {
		return err
	}
	clear(p.ehs[scope.ehLimit:])
	p.ehs = p.ehs[:scope.ehLimit]
	p.scopes = p.scopes[:newLevel]
	return nil
}

// Unlink each handler once. A refused decrement keeps only its ownership
// on the trail, so retry cannot unlink an older handler for the same term.
func (p *SMTRelevancyPropagator) undoTrail(limit int) error {
	for len(p.trail) > limit {
		i := len(p.trail) - 1
		entry := p.trail[i]
		var handler uint32
		switch entry.kind &^ smtRelevancyWatchOwnsParent {
		case smtRelevancyHandlerTrail:
			p.handlers.pop(entry.expression)
		case smtRelevancyPositiveWatchTrail:
			handler = p.positiveWatch.pop(entry.expression)
		case smtRelevancyNegativeWatchTrail:
			handler = p.negativeWatch.pop(entry.expression)
		case smtRelevancyRefTrail:
		default:
			return ast.ErrASTDomain
		}
		p.trail = p.trail[:i]
		if entry.kind&smtRelevancyWatchOwnsParent != 0 {
			// This was the last watch to unlink. Its trail slot now owns
			// just the parent, including if the source decrement fails.
			parent := p.ehs[handler-1].target
			p.trail = append(p.trail, smtRelevancyTrail{kind: smtRelevancyRefTrail, expression: parent})
		}
		consumed, err := p.context.manager.DecRefHeldConsumed(entry.expression)
		if !consumed {
			entry.kind = smtRelevancyRefTrail
			p.trail = append(p.trail, entry)
			if err == nil {
				return ast.ErrASTPending
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *SMTRelevancyPropagator) register(table *smtRelevancyTable, kind smtRelevancyTrailKind, source ast.ASTHandle, handler *smtRelevancyHandler) error {
	if handler.kind == smtRelevancyAssertedOr && handler.pooled == 0 {
		// The first watch owns the shared parent reference and is the last
		// watch removed when this scope is retired.
		if err := p.context.manager.IncRefPair(source, handler.target); err != nil {
			return err
		}
		kind |= smtRelevancyWatchOwnsParent
	} else if err := p.context.manager.IncRef(source); err != nil {
		return err
	}
	p.linkRetained(table, kind, source, handler)
	return nil
}

// The trail owns the source reference for each link. The first deferred
// watch of an asserted OR also owns the handler's parent reference.
func (p *SMTRelevancyPropagator) linkRetained(table *smtRelevancyTable, kind smtRelevancyTrailKind, source ast.ASTHandle, handler *smtRelevancyHandler) {
	p.trail = append(p.trail, smtRelevancyTrail{kind: kind, expression: source})
	table.push(source, p.pool(handler))
}

// release is ~relevancy_propagator_imp (context::flush): undo_trail(0)
// releases the trail newest first, then ~m_relevant_exprs releases the
// relevant expressions first to last.
func (p *SMTRelevancyPropagator) release() error {
	if p == nil || p.context == nil {
		return nil
	}
	if p.interruptedPop != 0 {
		if err := p.finishPop(p.interruptedPop-1, 0, 0, false); err != nil {
			return err
		}
		p.interruptedPop = 0
	}
	if err := p.undoTrail(0); err != nil {
		return err
	}
	for len(p.relevant) != 0 {
		expression := p.relevant[0]
		p.context.relevant.del(expression)
		consumed, err := p.context.manager.DecRefConsumed(expression)
		if consumed {
			p.relevant = p.relevant[1:]
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	p.relevant = nil
	p.newlyRelevant = nil
	p.newlyRelevantBase = 0
	p.scopes = nil
	p.ehs = nil
	p.qhead = 0
	return nil
}
