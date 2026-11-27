package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtCandidateMemo struct {
	enode    *SMTEnode
	hasEnode bool
	truth    [2]uint8 // absent, false or true
}

// A registration used by only one or two candidates stays on the lazy
// checker. Larger bodies and traced checks also use the ordinary AST walk.
func (stat *SMTQuantifierStat) prepareCandidate(m *ast.ASTManager, body ast.ASTHandle) *smtCandidateBody {
	const prepareAfter = 3
	if m.TraceStream != nil || m.Budget().InjectionArmed() {
		return nil
	}
	if stat.candidateChecks < prepareAfter {
		stat.candidateChecks++
		if stat.candidateChecks == prepareAfter {
			stat.candidateBody = prepareCandidateBody(m, body, stat.size)
		}
	}
	p := stat.candidateBody
	if p != nil && p.body == body {
		return p
	}
	return nil
}

func (c *SMTChecker) runCandidate(p *smtCandidateBody, want bool) (bool, error) {
	if cap(c.candidateMemo) < p.memoSize {
		c.candidateMemo = make([]smtCandidateMemo, p.memoSize)
	} else {
		c.candidateMemo = c.candidateMemo[:p.memoSize]
	}
	defer func() {
		clear(c.candidateMemo)
		c.bindings = nil
	}()
	// A live body owns all expressions in its prepared syntax. Only nodes
	// with memo slots need their individual reference counts below.
	if err := c.context.manager.ValidateHandle(p.body); err != nil {
		return false, err
	}
	return c.checkCandidate(p, 0, want)
}

func (c *SMTChecker) candidateMemoSlot(n *smtCandidateNode) (*smtCandidateMemo, error) {
	refs, err := c.context.manager.GetRefCount(n.expression)
	if err != nil || refs <= 1 {
		return nil, err
	}
	return &c.candidateMemo[n.memo-1], nil
}

func (c *SMTChecker) candidateEnode(p *smtCandidateBody, id uint16) (*SMTEnode, error) {
	n := &p.nodes[id]
	var slot *smtCandidateMemo
	if n.memo != 0 {
		var err error
		slot, err = c.candidateMemoSlot(n)
		if err != nil {
			return nil, err
		}
	}
	return c.candidateEnodeWithSlot(p, n, slot)
}

func (c *SMTChecker) candidateEnodeWithSlot(p *smtCandidateBody, n *smtCandidateNode, slot *smtCandidateMemo) (*SMTEnode, error) {
	if n.op == candidateVariable {
		if uint64(n.operand) >= uint64(len(c.bindings)) {
			return nil, nil
		}
		return c.bindings[len(c.bindings)-1-int(n.operand)], nil
	}
	if page := c.context.exprs.readPage(n.expression); page != nil {
		index := n.expression.Slot() % smtExprPageSize
		if node := page.enodes[index]; node != nil && c.context.recordRelevant(&page.records[index]) {
			return node, nil
		}
	}
	if n.first == n.end || n.op == candidateOpaque {
		return nil, nil
	}
	if slot != nil && slot.hasEnode {
		return slot.enode, nil
	}
	found, err := c.candidateAppEnode(p, n)
	if slot != nil && err == nil {
		slot.enode, slot.hasEnode = found, true
	}
	return found, err
}

func (c *SMTChecker) candidateAppEnode(p *smtCandidateBody, n *smtCandidateNode) (*SMTEnode, error) {
	ids := p.arguments[n.first:n.end]
	decl := &p.declarations[n.operand]
	if len(ids) <= 2 {
		var args [2]*SMTEnode
		for i, id := range ids {
			node, err := c.candidateEnode(p, id)
			if err != nil || node == nil {
				return nil, err
			}
			args[i] = node
		}
		return c.congruentEnodeWithFlags(decl.handle, decl.commutative, args[:len(ids)])
	}
	base := len(c.argBuffer)
	defer func() {
		clear(c.argBuffer[base:])
		c.argBuffer = c.argBuffer[:base]
	}()
	for _, id := range ids {
		node, err := c.candidateEnode(p, id)
		if err != nil || node == nil {
			return nil, err
		}
		c.argBuffer = append(c.argBuffer, node)
	}
	return c.congruentEnodeWithFlags(decl.handle, decl.commutative, c.argBuffer[base:])
}

func (c *SMTChecker) checkCandidate(p *smtCandidateBody, id uint16, want bool) (bool, error) {
	n := &p.nodes[id]
	var slot *smtCandidateMemo
	if n.memo != 0 {
		var err error
		slot, err = c.candidateMemoSlot(n)
		if err != nil {
			return false, err
		}
	}
	if slot == nil {
		return c.checkCandidateCore(p, n, slot, want)
	}
	polarity := 0
	if want {
		polarity = 1
	}
	if v := slot.truth[polarity]; v != 0 {
		return v == 2, nil
	}
	v, err := c.checkCandidateCore(p, n, slot, want)
	if err == nil {
		slot.truth[polarity] = 1
		if v {
			slot.truth[polarity] = 2
		}
	}
	return v, err
}

func (c *SMTChecker) checkCandidateCore(p *smtCandidateBody, n *smtCandidateNode, slot *smtCandidateMemo, want bool) (bool, error) {
	record := c.context.exprs.read(n.expression)
	if value, ok := c.context.assignments.lookupRecord(n.expression.Slot(), record); ok && c.context.recordRelevant(record) {
		if n.op == candidateNot || n.op == candidateFalse {
			value = c.context.getAssignment(n.expression)
		}
		return value != lbool.Undef && want == (value == lbool.True), nil
	}
	args := p.arguments[n.first:n.end]
	switch n.op {
	case candidateOpaque, candidateVariable:
		return false, nil
	case candidateTrue:
		return want, nil
	case candidateFalse:
		return !want, nil
	case candidateNot:
		return c.checkCandidate(p, args[0], !want)
	case candidateAnd, candidateOr:
		all := want == (n.op == candidateAnd)
		for _, arg := range args {
			match, err := c.checkCandidate(p, arg, want)
			if err != nil || match != all {
				return match, err
			}
		}
		return all, nil
	case candidateIff:
		left, err := c.checkCandidate(p, args[0], true)
		if err != nil {
			return false, err
		}
		if left {
			return c.checkCandidate(p, args[1], want)
		}
		leftFalse, err := c.checkCandidate(p, args[0], false)
		if err != nil || !leftFalse {
			return false, err
		}
		return c.checkCandidate(p, args[1], !want)
	case candidateIte:
		condition := &p.nodes[args[0]]
		if c.context.litInternalized(condition.expression) && c.context.isRelevant(condition.expression) {
			switch c.context.getAssignment(condition.expression) {
			case lbool.True:
				return c.checkCandidate(p, args[1], want)
			case lbool.False:
				return c.checkCandidate(p, args[2], want)
			default:
				return false, nil
			}
		}
		then, err := c.checkCandidate(p, args[1], want)
		if err != nil || !then {
			return false, err
		}
		return c.checkCandidate(p, args[2], want)
	case candidateEqual:
		left, err := c.candidateEnode(p, args[0])
		if err != nil {
			return false, err
		}
		right, err := c.candidateEnode(p, args[1])
		if err != nil {
			return false, err
		}
		return c.context.matchesEquality(p.declarations[n.operand].handle, left, right, want)
	default:
		node, err := c.candidateEnodeWithSlot(p, n, slot)
		if err != nil || node == nil || !node.boolean || !c.context.isRelevant(node.owner) {
			return false, err
		}
		value := c.context.getAssignment(node.owner)
		return value != lbool.Undef && want == (value == lbool.True), nil
	}
}
