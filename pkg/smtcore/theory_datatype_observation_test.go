package smtcore

import "github.com/glycerine/g3/pkg/ast"

// Test observation of all internalized recognizers, including unwatched ones.
type smtDatatypeRecognizer struct {
	expression ast.ASTHandle
	literal    SMTLiteral
}

func (t *SMTDatatypeTheory) recognizerSlots(value ast.ASTHandle) (map[ast.ASTHandle]smtDatatypeRecognizer, error) {
	s, err := t.State()
	if err != nil {
		return nil, err
	}
	node := s.context.enodes.get(value)
	if node == nil {
		return nil, ast.ErrASTDomain
	}
	slots := make(map[ast.ASTHandle]smtDatatypeRecognizer)
	for variable, expression := range s.boolExpr {
		if expression.IsNull() {
			continue
		}
		op, own, err := t.operation(expression)
		if err != nil {
			return nil, err
		}
		if !own || op != ast.ASTDatatypeRecognizer && op != ast.ASTDatatypeIs {
			continue
		}
		argument, err := s.manager.GetArg(expression, 0)
		if err != nil {
			return nil, err
		}
		other := s.context.enodes.get(argument)
		if other == nil || other.root != node.root {
			continue
		}
		decl, err := s.manager.GetDecl(expression)
		if err != nil {
			return nil, err
		}
		constructor, found, err := s.manager.DatatypeRecognizerConstructor(decl)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, ast.ErrASTDomain
		}
		if _, exists := slots[constructor]; !exists {
			slots[constructor] = smtDatatypeRecognizer{expression, NewSMTLiteral(int32(variable), false)}
		}
	}
	return slots, nil
}
