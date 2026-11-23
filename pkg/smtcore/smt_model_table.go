package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

// A tableModel compiles the reachable part of one candidate model. Its values
// are local integer IDs. Each concrete ID has a manager-owned model value.
// Additional IDs represent unnamed elements of an unfrozen uninterpreted sort.
type tableModel struct {
	manager     *ast.ASTManager
	model       *protomodel.ProtoModel
	values      []ast.ASTHandle
	sorts       []ast.ASTHandle
	valueIDs    map[ast.ASTHandle]int
	bySort      map[ast.ASTHandle][]int
	terms       []tableTerm
	termIDs     map[ast.ASTHandle]int
	functions   []tableFunction
	functionIDs map[ast.ASTHandle]int
	work        int
	incomplete  bool
}

type tableTerm struct {
	op    int32
	index int
	sort  ast.ASTHandle
	args  []int
}

type tableFunction struct {
	entries  []tableEntry
	fallback int
}

type tableEntry struct {
	keys  [tableMaxArity]int
	value int
}

const (
	tableVariable int32 = -1 - iota
	tableConstant
	tableApplication
)

const (
	tableFalse     = 1
	tableTrue      = 2
	tableMaxArity  = 8
	tableMaxTuples = 512
)

func (g *tableModel) valueID(value ast.ASTHandle) (int, error) {
	if id := g.valueIDs[value]; id != 0 {
		return id, nil
	}
	if value.IsNull() {
		return 0, nil
	}
	ok, err := g.manager.IsModelValue(value)
	if err != nil || !ok {
		return 0, err
	}
	sort, err := g.manager.GetSort(value)
	if err != nil {
		return 0, err
	}
	ok, err = g.manager.IsUninterpretedSort(sort)
	if err != nil || !ok {
		return 0, err
	}
	id := len(g.values)
	g.valueIDs[value] = id
	g.values = append(g.values, value)
	g.sorts = append(g.sorts, sort)
	g.bySort[sort] = append(g.bySort[sort], id)
	return id, nil
}

func tableOperator(kind int32, arity uint32) bool {
	switch kind {
	case ast.ASTOpAnd, ast.ASTOpOr:
		return true
	case ast.ASTOpEq, ast.ASTOpIff, ast.ASTOpImplies, ast.ASTOpXor:
		return arity == 2
	case ast.ASTOpNot:
		return arity == 1
	case ast.ASTOpIte:
		return arity == 3
	case ast.ASTOpDistinct:
		return arity <= 16
	}
	return false
}

func (g *tableModel) compile(term ast.ASTHandle, depth int) (int, error) {
	if term.IsNull() || depth > 48 || g.work <= 0 || len(g.terms) >= 768 {
		g.incomplete = true
		return 0, nil
	}
	g.work--
	if id, ok := g.termIDs[term]; ok {
		// A function interpretation may refer to itself, including indirectly.
		if id == 0 {
			g.incomplete = true
		}
		return id, nil
	}
	g.termIDs[term] = 0
	header, err := g.manager.NodeHeader(term)
	if err != nil {
		return 0, err
	}
	var n tableTerm
	switch header.Kind {
	case ast.ASTVar:
		index, err := g.manager.GetVarIndex(term)
		if err != nil {
			return 0, err
		}
		n.sort, err = g.manager.GetSort(term)
		if err != nil {
			return 0, err
		}
		n.op, n.index = tableVariable, int(index)
	case ast.ASTApp:
		if header.NumArgs > 80 {
			g.incomplete = true
			return 0, nil
		}
		if header.NumArgs == 0 {
			value := term
			if header.Family == ast.NullFamilyID {
				value, err = g.model.Core().ConstantInterpretation(header.Decl)
				if err != nil {
					return 0, err
				}
			}
			id, err := g.valueID(value)
			if err != nil {
				return 0, err
			}
			if id != 0 {
				sort, err := g.manager.GetSort(term)
				if err != nil {
					return 0, err
				}
				if g.sorts[id] != sort {
					g.incomplete = true
					return 0, nil
				}
				n.op, n.index = tableConstant, id
				break
			}
		}
		switch header.Family {
		case g.manager.BasicFamilyID():
			if !tableOperator(header.DeclKind, header.NumArgs) {
				g.incomplete = true
				return 0, nil
			}
			n.op = header.DeclKind
		case ast.NullFamilyID:
			if header.NumArgs == 0 {
				g.incomplete = true
				return 0, nil
			}
			n.index, err = g.function(header.Decl, header.NumArgs, depth+1)
			if err != nil || g.incomplete {
				return 0, err
			}
			n.op = tableApplication
		default:
			g.incomplete = true
			return 0, nil
		}
		n.args = make([]int, header.NumArgs)
		for i := range n.args {
			arg, err := g.manager.GetArg(term, uint32(i))
			if err != nil {
				return 0, err
			}
			n.args[i], err = g.compile(arg, depth+1)
			if err != nil || g.incomplete {
				return 0, err
			}
		}
	default:
		g.incomplete = true
		return 0, nil
	}
	id := len(g.terms)
	g.terms = append(g.terms, n)
	g.termIDs[term] = id
	return id, nil
}

func (g *tableModel) function(decl ast.ASTHandle, arity uint32, depth int) (int, error) {
	if id, ok := g.functionIDs[decl]; ok {
		if id < 0 {
			g.incomplete = true
		}
		return id, nil
	}
	if arity > tableMaxArity || len(g.functions) >= 180 {
		g.incomplete = true
		return 0, nil
	}
	g.functionIDs[decl] = -1
	f, err := g.model.Core().FunctionInterpretation(decl)
	if err != nil {
		return 0, err
	}
	if f == nil || f.Arity() != arity || !f.ArgumentsAreValues() || f.NumEntries() > 128 {
		g.incomplete = true
		return 0, nil
	}
	fallback, err := f.Else()
	if err != nil {
		return 0, err
	}
	if fallback.IsNull() {
		g.incomplete = true
		return 0, nil
	}
	resultSort, err := g.manager.GetRange(decl)
	if err != nil {
		return 0, err
	}
	sort, err := g.manager.GetSort(fallback)
	if err != nil {
		return 0, err
	}
	if sort != resultSort {
		g.incomplete = true
		return 0, nil
	}
	fn := tableFunction{entries: make([]tableEntry, f.NumEntries())}
	fn.fallback, err = g.compile(fallback, depth)
	if err != nil || g.incomplete {
		return 0, err
	}
	var domains [tableMaxArity]ast.ASTHandle
	for i := uint32(0); i < arity; i++ {
		domains[i], err = g.manager.GetDomain(decl, i)
		if err != nil {
			return 0, err
		}
	}
	// Symbolic and concrete source evaluation agree when each tuple has a
	// single result. A malformed or partial table goes to the auxiliary solver.
	var seen map[[tableMaxArity]int]struct{}
	if len(fn.entries) > 1 {
		seen = make(map[[tableMaxArity]int]struct{}, len(fn.entries))
	}
	for i := range fn.entries {
		entry, err := f.Entry(uint32(i))
		if err != nil {
			return 0, err
		}
		row := &fn.entries[i]
		for j := uint32(0); j < arity; j++ {
			value, err := entry.Argument(j)
			if err != nil {
				return 0, err
			}
			id, err := g.valueID(value)
			if err != nil {
				return 0, err
			}
			if id == 0 || g.sorts[id] != domains[j] {
				g.incomplete = true
				return 0, nil
			}
			row.keys[j] = id
		}
		if seen != nil {
			if _, ok := seen[row.keys]; ok {
				g.incomplete = true
				return 0, nil
			}
			seen[row.keys] = struct{}{}
		}
		value, err := entry.Result()
		if err != nil {
			return 0, err
		}
		row.value, err = g.valueID(value)
		if err != nil {
			return 0, err
		}
		if row.value == 0 || g.sorts[row.value] != resultSort {
			g.incomplete = true
			return 0, nil
		}
		g.work -= int(arity) + 1
		if g.work <= 0 {
			g.incomplete = true
			return 0, nil
		}
	}
	id := len(g.functions)
	g.functions = append(g.functions, fn)
	g.functionIDs[decl] = id
	return id, nil
}
