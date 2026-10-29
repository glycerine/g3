package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type pb2BVBooleanOracle struct {
	ids     map[ast.ASTHandle]int
	clauses [][]int
	count   int
}

func (q *pb2BVBooleanOracle) gate(t *testing.T, m *ast.ASTManager, h ast.ASTHandle) int {
	t.Helper()
	if id := q.ids[h]; id != 0 {
		return id
	}
	q.count++
	id := q.count
	q.ids[h] = id
	kind, e := m.GetKind(h)
	astTestMust(t, e)
	if kind != ast.ASTApp {
		return id
	}
	family, e := m.GetFamilyID(h)
	astTestMust(t, e)
	op, e := m.GetDeclKind(h)
	astTestMust(t, e)
	if family != m.BasicFamilyID() {
		return id
	}
	n, e := m.GetNumArgs(h)
	astTestMust(t, e)
	children := make([]int, n)
	for i := uint32(0); i < n; i++ {
		child, x := m.GetArg(h, i)
		astTestMust(t, x)
		children[i] = q.gate(t, m, child)
	}
	switch op {
	case ast.ASTOpTrue:
		q.clauses = append(q.clauses, []int{id})
	case ast.ASTOpFalse:
		q.clauses = append(q.clauses, []int{-id})
	case ast.ASTOpNot:
		q.clauses = append(q.clauses, []int{-id, -children[0]}, []int{id, children[0]})
	case ast.ASTOpAnd:
		back := []int{id}
		for _, child := range children {
			q.clauses = append(q.clauses, []int{-id, child})
			back = append(back, -child)
		}
		q.clauses = append(q.clauses, back)
	case ast.ASTOpOr:
		back := []int{-id}
		for _, child := range children {
			q.clauses = append(q.clauses, []int{id, -child})
			back = append(back, child)
		}
		q.clauses = append(q.clauses, back)
	default:
		t.Fatal("unsupported sorting oracle operation", op)
	}
	return id
}

func pb2BVOracleSAT(clauses [][]int, values []int8) bool {
	for {
		changed := false
		for _, clause := range clauses {
			satisfied, unknown, unit := false, 0, 0
			for _, literal := range clause {
				id := literal
				if id < 0 {
					id = -id
				}
				value := values[id]
				if value == 0 {
					unknown++
					unit = literal
				} else if literal > 0 && value > 0 || literal < 0 && value < 0 {
					satisfied = true
					break
				}
			}
			if satisfied {
				continue
			}
			if unknown == 0 {
				return false
			}
			if unknown == 1 {
				id, value := unit, int8(1)
				if id < 0 {
					id, value = -id, -1
				}
				if values[id] != 0 && values[id] != value {
					return false
				}
				if values[id] == 0 {
					values[id], changed = value, true
				}
			}
		}
		if !changed {
			break
		}
	}
	pick := 0
	for _, clause := range clauses {
		satisfied := false
		for _, literal := range clause {
			id := literal
			if id < 0 {
				id = -id
			}
			if literal > 0 && values[id] > 0 || literal < 0 && values[id] < 0 {
				satisfied = true
				break
			}
			if values[id] == 0 && pick == 0 {
				pick = id
			}
		}
		if !satisfied && pick != 0 {
			break
		}
	}
	if pick == 0 {
		return true
	}
	next := append([]int8(nil), values...)
	next[pick] = 1
	if pb2BVOracleSAT(clauses, next) {
		return true
	}
	next = append([]int8(nil), values...)
	next[pick] = -1
	return pb2BVOracleSAT(clauses, next)
}
