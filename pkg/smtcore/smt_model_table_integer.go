package smtcore

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

func tableIntegerOperator(kind int32, arity uint32) bool {
	switch kind {
	case smtArithOpLE, smtArithOpGE, smtArithOpLT, smtArithOpGT:
		return arity == 2
	case smtArithOpAdd, smtArithOpSub, smtArithOpMul:
		return arity >= 2
	case smtArithOpUMinus:
		return arity == 1
	}
	return false
}

// Numerals in a function interpretation and computed results use the same
// IDs, so equality and table lookup agree even without constructing an AST.
func (g *tableModel) integerID(number int32, sort ast.ASTHandle) int {
	if id := g.integerIDs[number]; id != 0 {
		return id
	}
	if len(g.integerIDs) >= 1024 {
		g.incomplete = true
		return 0
	}
	if g.integerIDs == nil {
		g.integerIDs = make(map[int32]int)
		g.integers = make(map[int]int32)
	}
	id := len(g.values)
	g.integerIDs[number], g.integers[id] = id, number
	g.values = append(g.values, ast.ASTHandle{})
	g.sorts = append(g.sorts, sort)
	return id
}

func (g *tableModel) evaluateInteger(term *tableTerm, bindings []int, depth int) int {
	first := g.evaluate(term.args[0], bindings, depth+1)
	value, ok := g.integers[first]
	if !ok {
		g.incomplete = true
		return 0
	}
	// Every operand and intermediate fits int32. The wider accumulator
	// therefore computes the exact result of each operation, including Mul.
	total := int64(value)
	kind := int32(term.index)
	if kind == smtArithOpUMinus {
		total = -total
	}
	for _, arg := range term.args[1:] {
		next, ok := g.integers[g.evaluate(arg, bindings, depth+1)]
		if !ok {
			g.incomplete = true
			return 0
		}
		right := int64(next)
		switch kind {
		case smtArithOpLE:
			return tableBool(total <= right)
		case smtArithOpGE:
			return tableBool(total >= right)
		case smtArithOpLT:
			return tableBool(total < right)
		case smtArithOpGT:
			return tableBool(total > right)
		case smtArithOpAdd:
			total += right
		case smtArithOpSub:
			total -= right
		case smtArithOpMul:
			total *= right
		}
		if total < math.MinInt32 || total > math.MaxInt32 {
			g.incomplete = true
			return 0
		}
	}
	if total > math.MaxInt32 { // Negation of MinInt32.
		g.incomplete = true
		return 0
	}
	return g.integerID(int32(total), g.sorts[first])
}
