package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// uniformModelBool proves a Boolean value independently of every binding.
// A predicate with no exceptional rows and a literal default has that value
// even when its arguments depend on the binders. An unknown operand can only
// be discarded when the connective's result is already determined.
func uniformModelBool(m *ast.ASTManager, model *protomodel.ProtoModel, expr, boolean ast.ASTHandle, remaining *int) lbool.LBool {
	if *remaining == 0 {
		return lbool.Undef
	}
	*remaining--
	if m.IsTrue(expr) {
		return lbool.True
	}
	if m.IsFalse(expr) {
		return lbool.False
	}
	h, err := m.NodeHeader(expr)
	if err != nil || h.Kind != ast.ASTApp {
		return lbool.Undef
	}
	if h.Family == ast.NullFamilyID {
		sort, err := m.GetRange(h.Decl)
		if err != nil || sort != boolean {
			return lbool.Undef
		}
		var value ast.ASTHandle
		if h.NumArgs == 0 {
			value, err = model.Core().ConstantInterpretation(h.Decl)
		} else {
			fn, err := model.Core().FunctionInterpretation(h.Decl)
			if err != nil || fn == nil || fn.Arity() != h.NumArgs || fn.NumEntries() != 0 {
				return lbool.Undef
			}
			value, err = fn.Else()
			if err != nil {
				return lbool.Undef
			}
		}
		if err == nil && m.IsTrue(value) {
			return lbool.True
		}
		if err == nil && m.IsFalse(value) {
			return lbool.False
		}
		return lbool.Undef
	}
	if h.Family != m.BasicFamilyID() || !tableOperator(h.DeclKind, h.NumArgs) {
		return lbool.Undef
	}
	child := func(i uint32) lbool.LBool {
		arg, err := m.GetArg(expr, i)
		if err != nil {
			return lbool.Undef
		}
		return uniformModelBool(m, model, arg, boolean, remaining)
	}
	switch h.DeclKind {
	case ast.ASTOpNot:
		return child(0).Negated()
	case ast.ASTOpAnd, ast.ASTOpOr:
		stop := lbool.FromBool(h.DeclKind == ast.ASTOpOr)
		result := stop.Negated()
		for i := uint32(0); i < h.NumArgs; i++ {
			if *remaining == 0 {
				return lbool.Undef
			}
			v := child(i)
			if v == stop {
				return v
			}
			if v == lbool.Undef {
				result = lbool.Undef
			}
		}
		return result
	case ast.ASTOpImplies:
		left := child(0)
		if left == lbool.False {
			return lbool.True
		}
		right := child(1)
		if right == lbool.True || left == lbool.True {
			return right
		}
	case ast.ASTOpEq, ast.ASTOpIff, ast.ASTOpXor:
		left := child(0)
		if left != lbool.Undef {
			if right := child(1); right != lbool.Undef {
				return lbool.FromBool((left == right) != (h.DeclKind == ast.ASTOpXor))
			}
		}
	case ast.ASTOpIte:
		switch child(0) {
		case lbool.True:
			return child(1)
		case lbool.False:
			return child(2)
		}
		if yes := child(1); yes != lbool.Undef && child(2) == yes {
			return yes
		}
	}
	return lbool.Undef
}
