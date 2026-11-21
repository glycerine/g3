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
	parent   *smtExprRecord
	children []smtAssertedOrChild
	witness  int
}

type smtAssertedOrChild struct {
	mark    *smtExprRecord
	negated *smtExprRecord // immediate argument of a syntactic NOT
}

func (p *SMTRelevancyPropagator) addAssertedOrWatches(parent ast.ASTHandle, args []ast.ASTHandle, literals []SMTLiteral) error {
	if !p.enabled || len(args) == 0 {
		return nil
	}
	for _, literal := range literals {
		if literal.Var() < SMTFirstBoolVar {
			return p.AddOrWatches(parent)
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
	gate.parent = p.context.exprs.write(parent)
	for i, literal := range literals {
		gate.children[i].mark = p.context.exprs.write(args[i])
		if literal.Sign() {
			operand := p.internalizer.boolExpr[literal.Var()]
			gate.children[i].negated = p.context.exprs.write(operand)
		}
	}
	handler := smtRelevancyHandler{kind: smtRelevancyAssertedOr, target: parent, assertedOr: gate}
	for _, child := range args {
		if err := p.addLiteralWatch(child, true, &handler); err != nil {
			return err
		}
	}
	return nil
}

func (p *SMTRelevancyPropagator) assertedOrChildValue(child smtAssertedOrChild) lbool.LBool {
	if child.negated != nil {
		value, _ := p.context.assignments.lookupRecord(child.negated)
		return value.Negated()
	}
	if child.mark.flags&smtExprHasBoolVar != 0 {
		return p.internalizer.assignment[child.mark.boolVar<<1]
	}
	value, _ := p.context.assignments.lookupRecord(child.mark)
	return value
}

func (p *SMTRelevancyPropagator) propagateAssertedOr(parent ast.ASTHandle, gate *smtAssertedOr) error {
	record := gate.parent
	if record.generation != parent.Generation() || record.flags&smtExprRelevant == 0 {
		return nil
	}
	if p.internalizer == nil || record.flags&smtExprHasBoolVar != 0 {
		// A later internalization can give the parent a gate and signed
		// Boolean literals. Use that gate's assignment rules while bound.
		return p.propagateOr(parent)
	}
	if record.flags&smtExprHasAssignment != 0 && lbool.LBool(record.assignment) == lbool.False {
		return p.propagateAll(parent)
	}
	if gate.witness >= 0 {
		child := gate.children[gate.witness]
		if child.mark.flags&smtExprRelevant != 0 && p.assertedOrChildValue(child) == lbool.True {
			return nil
		}
	}
	first := -1
	for i, child := range gate.children {
		if p.assertedOrChildValue(child) != lbool.True {
			continue
		}
		if child.mark.flags&smtExprRelevant != 0 {
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
