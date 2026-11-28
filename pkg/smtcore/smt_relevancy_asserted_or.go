package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// An asserted disjunction usually has no Boolean variable of its own.
// Its watches can still use the child bindings established during Assert.
// Expression records stay in their solver's pages, and the live parent AST
// owns its syntactic children. Each callback reads their current assignments.
type smtAssertedOr struct {
	children []smtAssertedOrChild
	witness  int
}

type smtAssertedOrChild struct {
	slot    uint32
	negated uint32 // immediate argument of a syntactic NOT, or zero
}

func (p *SMTRelevancyPropagator) addAssertedOrWatches(parent ast.ASTHandle, args []ast.ASTHandle, literals []SMTLiteral) (*smtAssertedOr, error) {
	if !p.enabled || len(args) == 0 {
		return nil, nil
	}
	for _, literal := range literals {
		if literal.Var() < SMTFirstBoolVar {
			return nil, p.addOrWatches(parent, smtRelevancyAssertedOr)
		}
	}
	var gate *smtAssertedOr
	if len(args) <= 4 {
		short := new(struct {
			gate     smtAssertedOr
			children [4]smtAssertedOrChild
		})
		gate = &short.gate
		gate.children = short.children[:len(args)]
	} else {
		gate = &smtAssertedOr{children: make([]smtAssertedOrChild, len(args))}
	}
	gate.witness = -1
	p.context.exprs.write(parent)
	for i, literal := range literals {
		p.context.exprs.write(args[i])
		gate.children[i].slot = args[i].Slot()
		if literal.Sign() {
			operand := p.internalizer.boolExpr[literal.Var()]
			p.context.exprs.write(operand)
			gate.children[i].negated = operand.Slot()
		}
	}
	handler := smtRelevancyHandler{kind: smtRelevancyAssertedOr, target: parent, assertedOr: gate}
	for _, child := range args {
		if err := p.addLiteralWatch(child, true, &handler); err != nil {
			return nil, err
		}
	}
	return gate, nil
}

// The asserted OR handler holds its parent through the trail. The parent
// keeps the child slots live, including each syntactic NOT's operand.
func (t *smtExprTable) recordAt(slot uint32) *smtExprRecord {
	page := t.pages[slot/smtExprPageSize]
	return &page.records[slot%smtExprPageSize]
}

func (p *SMTRelevancyPropagator) assertedOrChildValue(child smtAssertedOrChild) lbool.LBool {
	table := p.context.exprs
	if child.negated != 0 {
		record := table.recordAt(child.negated)
		value, _ := p.context.assignments.lookupRecord(child.negated, record)
		return value.Negated()
	}
	record := table.recordAt(child.slot)
	if record.flags&smtExprHasBoolVar != 0 {
		return p.internalizer.assignment[table.variableAt(child.slot)<<1]
	}
	value, _ := p.context.assignments.lookupRecord(child.slot, record)
	return value
}

func (p *SMTRelevancyPropagator) propagateAssertedOr(parent ast.ASTHandle, gate *smtAssertedOr) error {
	record := p.context.exprs.read(parent)
	if record == nil || record.flags&smtExprRelevant == 0 {
		return nil
	}
	return p.propagateOrWithBindings(parent, gate, record)
}

// A queued visit still runs if a prior callback cleared the parent's mark.
// Only the assignment-watch entry point above requires a current mark.
func (p *SMTRelevancyPropagator) propagateOrWithBindings(parent ast.ASTHandle, gate *smtAssertedOr, record *smtExprRecord) error {
	if gate == nil || p.internalizer == nil || record == nil || record.flags&smtExprHasBoolVar != 0 {
		// A later internalization can give the parent a gate and signed
		// Boolean literals. Use that gate's assignment rules while bound.
		return p.propagateOr(parent)
	}
	if record.flags&smtExprHasAssignment != 0 && lbool.LBool(record.assignment) == lbool.False {
		return p.propagateAll(parent)
	}
	if gate.witness >= 0 {
		child := gate.children[gate.witness]
		if p.context.exprs.recordAt(child.slot).flags&smtExprRelevant != 0 && p.assertedOrChildValue(child) == lbool.True {
			return nil
		}
	}
	first := -1
	for i, child := range gate.children {
		if p.assertedOrChildValue(child) != lbool.True {
			continue
		}
		if p.context.exprs.recordAt(child.slot).flags&smtExprRelevant != 0 {
			gate.witness = i
			return nil
		}
		if first < 0 {
			first = i
		}
	}
	if first < 0 {
		return nil
	}
	argument, err := p.context.manager.GetArg(parent, uint32(first))
	if err != nil {
		return err
	}
	gate.witness = first
	return p.MarkRelevant(argument)
}
