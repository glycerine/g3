package smtcore

import (
	"slices"

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
		if uint64(term.index) >= uint64(len(bindings)) {
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
		return int(term.index)
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
	case tableArithmetic:
		return g.evaluateInteger(term, bindings, depth)
	case tableApplication:
		var args [tableMaxArity]int
		for i := range term.args {
			args[i] = eval(i)
		}
		fn := &g.functions[term.index]
		// Both key arrays are zero-padded. The first two positions are the
		// complete key for unary and binary functions.
		for i := range fn.entries {
			row := &fn.entries[i]
			if row.keys[0] == args[0] && row.keys[1] == args[1] && (len(term.args) <= 2 || row.keys == args) {
				g.work -= i + 1
				return row.value
			}
		}
		g.work -= len(fn.entries)
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
func (g *tableModel) check(m *ast.ASTManager, model *protomodel.ProtoModel, flat, body ast.ASTHandle, count uint32, checker *SMTModelChecker) (lbool.LBool, []ast.ASTHandle, error) {
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
	limit := m.ResourceLimit()
	bounded := limit.Limit != 0
	if bounded && !limit.IncOffset(32) {
		return lbool.Undef, nil, nil
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
	// On small integer-valued tables the ordinary search is cheap and its
	// choice of witnesses benefits later checks on the same solver.
	if len(g.integers) != 0 && tuples < 64 {
		return lbool.Undef, nil, nil
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
		if j%16 == 0 && (limit.CancelFlag() || bounded && !limit.IncOffset(uint32(min(16, tuples-j)))) {
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
				if !bounded && checker != nil && count == 1 && len(checker.newInstances) != 0 {
					set, err := checker.modelFinder.instantiationSet(flat, 0, false)
					if err != nil {
						return lbool.Undef, nil, err
					}
					if set != nil {
						witness[0], err = g.unaryWitness(root, domains[0], set, checker.newInstances, witness[0])
						if err != nil {
							return lbool.Undef, nil, err
						}
					}
				}
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

// Extra auxiliary premises require the ordinary search. Only private finite
// refutations opt into directly paying for table checks under a resource limit.
// With internalized arithmetic, its witness order also benefits mixed problems.
func (c *SMTModelChecker) canCheckModelTables() bool {
	m, aux := c.manager, c.auxiliaryContext
	if aux.manager != m || aux.ScopeLevelCore() != 0 || len(aux.tracked) != 0 || len(aux.kernel.assertions) != 0 ||
		aux.kernel.smtState.MaxConflicts != ^uint32(0) || c.trace || m.TraceStream != nil ||
		m.Budget().InjectionArmed() || (m.ResourceLimit().Limit != 0 && !aux.config.CertifyBoundedModelTables) || m.ResourceLimit().CancelFlag() {
		return false
	}
	// The compiler checks the sorts, operators and function interpretations
	// reachable from this quantifier. Other assertions can use arithmetic.
	return true
}

// When two unary instances would bind the same derived term, try to cover a
// second term in this batch. An alternative must falsify the compiled body
// within its remaining evaluation budget and have an unused inverse binding.
func (g *tableModel) unaryWitness(root int, domain []int, set *smtModelFinderInstantiationSet, pending []smtModelCheckerInstance, original ast.ASTHandle) (ast.ASTHandle, error) {
	// Bound the scan even when a model check has queued many quantifiers.
	const lookback = 32
	if len(pending) > lookback {
		pending = pending[len(pending)-lookback:]
	}
	used := func(term ast.ASTHandle) bool {
		for _, instance := range pending {
			if len(instance.bindings) == 1 && instance.bindings[0] == term {
				return true
			}
		}
		return false
	}
	term, generation, found, err := set.Inverse(original)
	if err != nil || !found || generation == 0 || !used(term) {
		return original, err
	}
	for _, value := range set.inverse.FirstKeys(min(8, set.inverse.Len())) {
		id := g.valueIDs[value]
		if value == original || id == 0 || !slices.Contains(domain, id) {
			continue
		}
		truth := g.evaluate(root, []int{id}, 0)
		if g.incomplete || g.work <= 0 {
			break
		}
		if truth != tableFalse {
			continue
		}
		term, _, found, err := set.Inverse(value)
		if err != nil {
			return original, err
		}
		if found && !used(term) {
			return value, nil
		}
	}
	return original, nil
}
