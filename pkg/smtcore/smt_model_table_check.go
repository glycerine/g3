package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func tableBool(value bool) int {
	if value {
		return tableTrue
	}
	return tableFalse
}

func (g *tableModel) evaluate(id int, bindings []int, depth int) int {
	g.work--
	if g.work <= 0 || depth > 64 {
		g.incomplete = true
		return 0
	}
	term := &g.terms[id]
	eval := func(i int) int { return g.evaluate(term.args[i], bindings, depth+1) }
	switch term.op {
	case tableVariable:
		if term.index >= len(bindings) {
			g.incomplete = true
			return 0
		}
		value := bindings[term.index]
		if value == 0 || g.sorts[value] != term.sort {
			g.incomplete = true
			return 0
		}
		return value
	case tableConstant:
		return term.index
	case ast.ASTOpNot:
		return tableBool(eval(0) == tableFalse)
	case ast.ASTOpAnd, ast.ASTOpOr:
		stop := tableBool(term.op == ast.ASTOpOr)
		for i := range term.args {
			if eval(i) == stop {
				return stop
			}
		}
		return tableBool(stop == tableFalse)
	case ast.ASTOpImplies:
		return tableBool(eval(0) == tableFalse || eval(1) == tableTrue)
	case ast.ASTOpIte:
		if eval(0) == tableTrue {
			return eval(1)
		}
		return eval(2)
	case ast.ASTOpIff, ast.ASTOpEq:
		return tableBool(eval(0) == eval(1))
	case ast.ASTOpXor:
		return tableBool(eval(0) != eval(1))
	case ast.ASTOpDistinct:
		var previous [16]int
		for i := range term.args {
			value := eval(i)
			for _, other := range previous[:i] {
				if value == other {
					return tableFalse
				}
			}
			previous[i] = value
		}
		return tableTrue
	case tableApplication:
		var args [tableMaxArity]int
		for i := range term.args {
			args[i] = eval(i)
		}
		fn := &g.functions[term.index]
		for _, entry := range fn.entries {
			g.work--
			if entry.keys == args {
				return entry.value
			}
		}
		// Model function else arguments use direct order (variable 0 is
		// the first argument), as in FunctionInterpretation.EvalElse.
		return g.evaluate(fn.fallback, args[:len(term.args)], depth+1)
	}
	g.incomplete = true
	return 0
}

// A frozen sort has exactly its registered universe. Otherwise any permutation
// fixing every named element commutes with equality, ITE and these function
// tables. For k binders of one sort, at most k distinct unnamed elements
// occur in any assignment. Adding k representatives therefore covers every
// equality pattern, even when the quantifier compares several unseen values.
func (g *tableModel) domain(sort ast.ASTHandle, boolean ast.ASTHandle, binders int) ([]int, error) {
	if sort == boolean {
		return []int{tableFalse, tableTrue}, nil
	}
	frozen, err := g.model.IsFinite(sort)
	if err != nil {
		return nil, err
	}
	universe, err := g.model.KnownUniverse(sort)
	if err != nil || len(universe) > 64 {
		return nil, err
	}
	domain := make([]int, 0, len(universe))
	for _, value := range universe {
		id, err := g.valueID(value)
		if err != nil || id == 0 || g.sorts[id] != sort {
			return nil, err
		}
		domain = append(domain, id)
	}
	if frozen {
		return domain, nil
	}
	domain = append(domain[:0], g.bySort[sort]...)
	for i := 0; i < binders; i++ {
		domain = append(domain, len(g.values))
		g.values = append(g.values, ast.ASTHandle{})
		g.sorts = append(g.sorts, sort)
	}
	return domain, nil
}

// check reports whether the negated body has a concrete witness.
// Undef delegates to ordinary MBQI, including false tuples for which only
// abstract representatives are available.
func (g *tableModel) check(m *ast.ASTManager, model *protomodel.ProtoModel, flat, body ast.ASTHandle, count uint32) (lbool.LBool, []ast.ASTHandle, error) {
	if model.Core().Manager() != m || count == 0 || count > 6 || m.ResourceLimit().CancelFlag() {
		return lbool.Undef, nil, nil
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return lbool.Undef, nil, err
	}
	sorts := make([]ast.ASTHandle, count)
	multiplicity := make(map[ast.ASTHandle]int)
	for i := range sorts {
		sort, err := m.GetDeclarationSort(flat, uint32(i))
		if err != nil {
			return lbool.Undef, nil, err
		}
		if sort != boolean {
			ok, err := m.IsUninterpretedSort(sort)
			if err != nil || !ok {
				return lbool.Undef, nil, err
			}
		}
		sorts[i] = sort
		multiplicity[sort]++
	}
	no, err := m.MkFalse()
	if err != nil {
		return lbool.Undef, nil, err
	}
	yes, err := m.MkTrue()
	if err != nil {
		return lbool.Undef, nil, err
	}
	g.manager, g.model, g.work = m, model, 6000
	defer g.clearModel()
	if g.valueIDs == nil {
		g.valueIDs = make(map[ast.ASTHandle]int)
		g.termIDs = make(map[ast.ASTHandle]int)
		g.functionIDs = make(map[ast.ASTHandle]int)
		g.bySort = make(map[ast.ASTHandle][]int)
	}
	g.valueIDs[no], g.valueIDs[yes] = tableFalse, tableTrue
	g.values = append(g.values, ast.ASTHandle{}, no, yes)
	g.sorts = append(g.sorts, ast.ASTHandle{}, boolean, boolean)
	g.terms = append(g.terms, tableTerm{})
	root, err := g.compile(body, 0)
	if err != nil || g.incomplete {
		return lbool.Undef, nil, err
	}
	domains := make([][]int, count)
	bySort := make(map[ast.ASTHandle][]int)
	tuples := 1
	for i, sort := range sorts {
		domain, ok := bySort[sort]
		if !ok {
			domain, err = g.domain(sort, boolean, multiplicity[sort])
			if err != nil {
				return lbool.Undef, nil, err
			}
			bySort[sort] = domain
		}
		if len(domain) == 0 || len(domain) > 64 {
			return lbool.Undef, nil, nil
		}
		domains[i] = domain
		if tuples <= tableMaxTuples {
			tuples *= len(domain)
		}
	}
	if tuples > tableMaxTuples {
		status, witness := g.checkDiagram(root, domains)
		return status, witness, nil
	}
	g.work = 100000
	positions := make([]int, count)
	bindings := make([]int, count)
	hasAbstractWitness := false
	for j := 0; j < tuples; j++ {
		if j%16 == 0 && m.ResourceLimit().CancelFlag() {
			return lbool.Undef, nil, nil
		}
		// Quantified declarations use the opposite order from de Bruijn indices.
		for i, domain := range domains {
			bindings[len(bindings)-1-i] = domain[positions[i]]
		}
		value := g.evaluate(root, bindings, 0)
		if g.incomplete || g.work <= 0 {
			return lbool.Undef, nil, nil
		}
		switch value {
		case tableTrue:
		case tableFalse:
			if witness := g.witness(bindings); witness != nil {
				return lbool.True, witness, nil
			}
			hasAbstractWitness = true
		default:
			return lbool.Undef, nil, nil
		}
		for i := len(positions) - 1; i >= 0; i-- {
			positions[i]++
			if positions[i] < len(domains[i]) {
				break
			}
			positions[i] = 0
		}
	}
	if hasAbstractWitness {
		return lbool.Undef, nil, nil
	}
	return lbool.False, nil, nil
}

func (g *tableModel) witness(bindings []int) []ast.ASTHandle {
	for _, id := range bindings {
		if g.values[id].IsNull() {
			return nil
		}
	}
	values := make([]ast.ASTHandle, len(bindings))
	for i, id := range bindings {
		values[len(values)-1-i] = g.values[id]
	}
	return values
}

// Extra auxiliary premises or resource limits require the ordinary search.
// With internalized arithmetic, its witness order also benefits mixed problems.
func (c *SMTModelChecker) canCheckModelTables() bool {
	m, aux := c.manager, c.auxiliaryContext
	if aux.manager != m || aux.ScopeLevelCore() != 0 || len(aux.tracked) != 0 || len(aux.kernel.assertions) != 0 ||
		aux.kernel.smtState.MaxConflicts != ^uint32(0) || c.trace || m.TraceStream != nil ||
		m.Budget().InjectionArmed() || m.ResourceLimit().Limit != 0 || m.ResourceLimit().CancelFlag() {
		return false
	}
	for _, theory := range c.state.theories {
		if arith, ok := theory.(*SMTTheoryArith); ok && (len(arith.atoms) > 0 || len(arith.varPos) > 0) {
			return false
		}
	}
	return true
}
