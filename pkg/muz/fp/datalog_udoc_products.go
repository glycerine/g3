package fp

// Source: dl_relation_manager.cpp mixed join fallback, dl_product_relation.cpp,
// and instr_io/instr_union's inference of an empty target from its source kind.
// This public consumer accepts extensional unary DOC plus a finite Bool/BV
// input, one acyclic rule per mixed head, and query auxiliaries. Ground rules
// may populate unary DOC, all-Bool finite inputs, and unseeded Bool/BV products
// assembled under favourite DOC; broader programs keep the existing finite path.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type udocProductFilter struct {
	column uint32
	value  ast.ASTHandle
}
type udocProductRulePlan struct {
	head, doc, finite, source ast.ASTHandle
	pair                      *udocProductPairPlan
	docFirst                  bool
	docColumns, finiteColumns []uint32
	filters                   []udocProductFilter
	identical                 [][]uint32
	removed                   []uint32
	cycles                    [][]uint32
	output                    []uint32
	inputKind                 uint8 // 0 product/mixed, 1 original DOC, 2 original finite
	ruleIndex                 int
}
type udocProductProgram struct {
	rules          map[ast.ASTHandle]*udocProductRulePlan
	sorts          map[ast.ASTHandle][]ast.ASTHandle
	finiteSorts    map[ast.ASTHandle][]ast.ASTHandle
	docs, finite   map[ast.ASTHandle]bool
	order          []ast.ASTHandle
	groundFacts    []*udocProductGroundRule
	groundProducts map[ast.ASTHandle]int // Mixed ground input -> its BV column.
}

func (p *udocProductProgram) productHead(d ast.ASTHandle) bool {
	if _, found := p.groundProducts[d]; found {
		return true
	}
	plan := p.rules[d]
	return plan != nil && plan.inputKind == 0
}

func udocProductFiniteKind(table rel.FiniteTable) string {
	switch table.(type) {
	case *rel.HashTable:
		return "hashtable"
	case *rel.SparseTable:
		return "sparse"
	case *rel.LazySparseTable:
		return "lazy_sparse"
	}
	return ""
}

func productArguments(m *ast.ASTManager, app ast.ASTHandle) ([]ast.ASTHandle, error) {
	count, err := m.GetNumArgs(app)
	if err != nil {
		return nil, err
	}
	args := make([]ast.ASTHandle, count)
	for i := range args {
		args[i], err = m.GetArg(app, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return args, nil
}

func planUDocProductAssembly(m *ast.ASTManager, plan *udocProductRulePlan, arguments []ast.ASTHandle, head ast.ASTHandle, allowConstants bool) error {
	variables := make(map[uint32]uint32)
	for column, argument := range arguments {
		kind, err := m.GetKind(argument)
		if err != nil {
			return err
		}
		if kind != ast.ASTVar {
			value, err := m.IsValue(argument)
			if err != nil {
				return err
			}
			if !allowConstants || !value {
				return ErrUDocDatalogFragment
			}
			plan.filters = append(plan.filters, udocProductFilter{uint32(column), argument})
			continue
		}
		index, err := m.GetVarIndex(argument)
		if err != nil {
			return err
		}
		if prior, found := variables[index]; found {
			if allowConstants {
				plan.identical = append(plan.identical, []uint32{prior, uint32(column)})
			}
		} else {
			variables[index] = uint32(column)
		}
	}
	args, err := productArguments(m, head)
	if err != nil {
		return err
	}
	wanted, used := make([]uint32, len(args)), make(map[uint32]bool)
	for i, arg := range args {
		kind, err := m.GetKind(arg)
		if err != nil {
			return err
		}
		if kind != ast.ASTVar {
			return ErrUDocDatalogFragment
		}
		index, err := m.GetVarIndex(arg)
		if err != nil {
			return err
		}
		column, found := variables[index]
		if !found || used[column] {
			return ErrUDocDatalogFragment
		}
		wanted[i], used[column] = column, true
	}
	var current []uint32
	for column := range arguments {
		if used[uint32(column)] {
			current = append(current, uint32(column))
		} else {
			plan.removed = append(plan.removed, uint32(column))
		}
	}
	for destination, want := range wanted {
		position := destination
		for current[position] != want {
			position++
		}
		if position != destination {
			plan.cycles = append(plan.cycles, []uint32{uint32(destination), uint32(position)})
			current[destination], current[position] = current[position], current[destination]
		}
	}
	plan.output = wanted
	return nil
}

func (e *PropositionalEngine) planUDocProducts(factory *rel.UDocRelationFactory, docFavourite bool, known map[ast.ASTHandle][]ast.ASTHandle) (program *udocProductProgram, resultErr error) {
	if factory.FormulaChecked() {
		return nil, nil
	}
	count, err := e.set.Len()
	if err != nil {
		return nil, err
	}
	var rules []*base.Rule
	defer func() {
		var cleanup error
		for _, rule := range rules {
			cleanup = errors.Join(cleanup, rule.Close())
		}
		if errors.Is(resultErr, ErrUDocDatalogFragment) && cleanup == nil {
			program, resultErr = nil, nil
		} else {
			resultErr = errors.Join(resultErr, cleanup)
		}
	}()
	definitions := make(map[ast.ASTHandle]int)
	groundRules := make(map[int]*udocProductGroundRule)
	groundHeads := make(map[ast.ASTHandle]bool)
	forwardHeads := make(map[ast.ASTHandle]bool)
	pairHeads := make(map[ast.ASTHandle]bool)
	for i := 0; i < count; i++ {
		rule, err := e.set.Rule(i)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
		fact, err := udocProductGroundFact(e.manager, rule)
		if err != nil {
			return nil, err
		}
		if fact != nil {
			groundRules[i], groundHeads[fact.head] = fact, true
			continue
		}
		head, err := rule.HeadDeclaration()
		if err != nil {
			return nil, err
		}
		definitions[head]++
	}
	p := &udocProductProgram{rules: make(map[ast.ASTHandle]*udocProductRulePlan), sorts: make(map[ast.ASTHandle][]ast.ASTHandle), finiteSorts: make(map[ast.ASTHandle][]ast.ASTHandle), docs: make(map[ast.ASTHandle]bool), finite: make(map[ast.ASTHandle]bool), groundProducts: make(map[ast.ASTHandle]int)}
	boolean, err := e.manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	isDoc := func(d ast.ASTHandle) (bool, error) {
		state := e.predicates[d]
		if state == nil || state.external != nil || state.explicitFiniteRepresentation || len(state.sorts) != 1 || !e.relations.CanHandleTableSorts(state.sorts) {
			return false, nil
		}
		selected := len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc" || docFavourite && len(state.requestedRepresentations) == 0
		if !selected {
			return false, nil
		}
		return factory.CanHandleSignature(state.sorts)
	}
	for ruleIndex, rule := range rules {
		if groundRules[ruleIndex] != nil {
			continue
		}
		head, err := rule.HeadDeclaration()
		if err != nil {
			return nil, err
		}
		if e.auxiliary[head] {
			continue
		}
		state := e.predicates[head]
		if definitions[head] != 1 || groundHeads[head] || state == nil || state.external != nil || len(state.requestedRepresentations) != 0 || !e.relations.CanHandleTableSorts(state.sorts) {
			return nil, ErrUDocDatalogFragment
		}
		hasBool, hasOther := false, false
		for _, sort := range state.sorts {
			hasBool = hasBool || sort == boolean
			hasOther = hasOther || sort != boolean
		}
		if !hasBool || !hasOther {
			return nil, ErrUDocDatalogFragment
		}
		if known[head] == nil && state.table != nil {
			rows, err := state.table.Len()
			if err != nil {
				return nil, err
			}
			if rows != 0 {
				return nil, ErrUDocDatalogFragment
			}
		}
		positive, err := rule.PositiveTailSize()
		if err != nil {
			return nil, err
		}
		total, err := rule.TailSize()
		if err != nil {
			return nil, err
		}
		if positive == 1 && total == 1 {
			// Source acyclic strata forward an existing product through
			// filtering and head assembly. Keep ordinary targets mixed so
			// their later unions still have two nontransparent components.
			for _, sort := range state.sorts {
				if sort == boolean {
					continue
				}
				bitVector, err := e.manager.IsBitVectorSort(sort)
				if err != nil {
					return nil, err
				}
				if !bitVector {
					return nil, ErrUDocDatalogFragment
				}
			}
			forwardHeads[head] = true
			continue
		}
		if positive != 2 || total != 2 {
			return nil, ErrUDocDatalogFragment
		}
		tails, decls := make([]ast.ASTHandle, 2), make([]ast.ASTHandle, 2)
		for i := range tails {
			tails[i], _, err = rule.Tail(i)
			if err != nil {
				return nil, err
			}
			decls[i], err = e.manager.GetDecl(tails[i])
			if err != nil {
				return nil, err
			}
		}
		leftDoc, err := isDoc(decls[0])
		if err != nil {
			return nil, err
		}
		rightDoc, err := isDoc(decls[1])
		if err != nil {
			return nil, err
		}
		if leftDoc == rightDoc {
			if leftDoc {
				return nil, ErrUDocDatalogFragment
			}
			// Both producers may acquire product kinds later in dependency
			// order. This slice keeps their ordinary target mixed BV/Bool,
			// with the same concrete union boundary as product forwarding.
			for _, sort := range state.sorts {
				if sort == boolean {
					continue
				}
				bitVector, err := e.manager.IsBitVectorSort(sort)
				if err != nil {
					return nil, err
				}
				if !bitVector {
					return nil, ErrUDocDatalogFragment
				}
			}
			pairHeads[head] = true
			continue
		}
		di, ti := 0, 1
		if rightDoc {
			di, ti = 1, 0
		}
		doc, finite := decls[di], decls[ti]
		input := e.predicates[finite]
		if definitions[doc] != 0 || definitions[finite] != 0 || input == nil || input.external != nil || !e.relations.CanHandleTableSorts(input.sorts) {
			return nil, ErrUDocDatalogFragment
		}
		if input.table == nil && known[finite] == nil || input.table != nil && udocProductFiniteKind(input.table) != e.datalogDefaultTable {
			return nil, ErrUDocDatalogFragment
		}
		if len(input.requestedRepresentations) != 0 && !input.explicitFiniteRepresentation {
			return nil, ErrUDocDatalogFragment
		}
		hasBool, hasOther = false, false
		for _, sort := range input.sorts {
			hasBool = hasBool || sort == boolean
			hasOther = hasOther || sort != boolean
		}
		if !hasBool {
			return nil, ErrUDocDatalogFragment
		}
		mixedGround := groundHeads[finite] && hasOther
		if mixedGround {
			// A mixed ground tuple is assembled from singletons. Requiring
			// favourite DOC and no predicate override makes every cached BV
			// singleton choose DOC, regardless of its first source consumer.
			if !docFavourite || e.datalogRelationFavourite != "doc" || len(input.sorts) != 2 || len(input.requestedRepresentations) != 0 || input.explicitFiniteRepresentation {
				return nil, ErrUDocDatalogFragment
			}
			bitColumn := 0
			if input.sorts[0] == boolean {
				bitColumn = 1
			}
			bitVector, err := e.manager.IsBitVectorSort(input.sorts[bitColumn])
			if err != nil {
				return nil, err
			}
			if !bitVector {
				return nil, ErrUDocDatalogFragment
			}
			if known[finite] == nil && input.table != nil {
				rows, err := input.table.Len()
				if err != nil {
					return nil, err
				}
				if rows != 0 {
					return nil, ErrUDocDatalogFragment
				}
			}
			p.groundProducts[finite] = bitColumn
			p.sorts[finite] = append([]ast.ASTHandle(nil), input.sorts...)
		}
		da, err := productArguments(e.manager, tails[di])
		if err != nil {
			return nil, err
		}
		fa, err := productArguments(e.manager, tails[ti])
		if err != nil {
			return nil, err
		}
		plan := &udocProductRulePlan{head: head, doc: doc, finite: finite, docFirst: di == 0}
		for _, args := range [][]ast.ASTHandle{da, fa} {
			seen := make(map[ast.ASTHandle]bool)
			for _, arg := range args {
				kind, err := e.manager.GetKind(arg)
				if err != nil {
					return nil, err
				}
				if kind != ast.ASTVar || seen[arg] {
					return nil, ErrUDocDatalogFragment
				}
				seen[arg] = true
			}
		}
		for i, darg := range da {
			for j, farg := range fa {
				if darg == farg {
					plan.docColumns = append(plan.docColumns, uint32(i))
					plan.finiteColumns = append(plan.finiteColumns, uint32(j))
				}
			}
		}
		arguments := append(append([]ast.ASTHandle(nil), da...), fa...)
		if di == 1 {
			arguments = append(append([]ast.ASTHandle(nil), fa...), da...)
		}
		h, err := rule.Head()
		if err != nil {
			return nil, err
		}
		if err = planUDocProductAssembly(e.manager, plan, arguments, h, false); err != nil {
			return nil, err
		}
		p.rules[head], p.sorts[head], p.docs[doc] = plan, append([]ast.ASTHandle(nil), state.sorts...), true
		if !mixedGround {
			p.finite[finite] = true
		}
		p.finiteSorts[finite] = append([]ast.ASTHandle(nil), input.sorts...)
		p.order = append(p.order, head)
	}
	if len(p.order) == 0 {
		return nil, nil
	}
	// Only all-Bool finite tuples reduce to ordinary table unions; admitted
	// mixed tuples must execute their source singleton product assembly.
	for ruleIndex := range rules {
		fact := groundRules[ruleIndex]
		if fact == nil {
			continue
		}
		_, mixed := p.groundProducts[fact.head]
		if !p.docs[fact.head] && !mixed {
			if !p.finite[fact.head] {
				return nil, ErrUDocDatalogFragment
			}
			for _, sort := range p.finiteSorts[fact.head] {
				if sort != boolean {
					return nil, ErrUDocDatalogFragment
				}
			}
		}
		p.groundFacts = append(p.groundFacts, fact)
	}
	for pending := true; pending; {
		pending = false
		advanced := false
		for ruleIndex, rule := range rules {
			if groundRules[ruleIndex] != nil {
				continue
			}
			head, err := rule.HeadDeclaration()
			if err != nil {
				return nil, err
			}
			if p.rules[head] != nil {
				continue
			}
			pending = true
			positive, err := rule.PositiveTailSize()
			if err != nil {
				return nil, err
			}
			total, err := rule.TailSize()
			if err != nil {
				return nil, err
			}
			if pairHeads[head] {
				if definitions[head] != 1 || positive != 2 || total != 2 {
					return nil, ErrUDocDatalogFragment
				}
				plan, err := e.planUDocProductPair(p, rule)
				if err != nil {
					return nil, err
				}
				if plan == nil {
					continue
				}
				p.rules[head], p.sorts[head] = plan, append([]ast.ASTHandle(nil), e.predicates[head].sorts...)
				p.order = append(p.order, head)
				advanced = true
				continue
			}
			if !e.auxiliary[head] && !forwardHeads[head] || definitions[head] != 1 || positive != 1 || total != 1 {
				return nil, ErrUDocDatalogFragment
			}
			tail, _, err := rule.Tail(0)
			if err != nil {
				return nil, err
			}
			source, err := e.manager.GetDecl(tail)
			if err != nil {
				return nil, err
			}
			if forwardHeads[head] && !p.productHead(source) {
				// A later rule may establish the source's product kind. The
				// no-progress check below rejects recursive or unresolved
				// dependencies; generated DOC/finite auxiliaries stay separate.
				if p.rules[source] != nil || p.docs[source] || p.finite[source] {
					return nil, ErrUDocDatalogFragment
				}
				continue
			}
			if !p.productHead(source) && p.rules[source] == nil && !p.docs[source] && !p.finite[source] {
				continue
			}
			plan := &udocProductRulePlan{head: head, source: source, ruleIndex: ruleIndex}
			if p.docs[source] {
				plan.inputKind = 1
			}
			if p.finite[source] {
				plan.inputKind = 2
			}
			if prior := p.rules[source]; prior != nil {
				plan.inputKind = prior.inputKind
			}
			args, err := productArguments(e.manager, tail)
			if err != nil {
				return nil, err
			}
			h, err := rule.Head()
			if err != nil {
				return nil, err
			}
			if err = planUDocProductAssembly(e.manager, plan, args, h, true); err != nil {
				return nil, err
			}
			p.rules[head], p.sorts[head] = plan, append([]ast.ASTHandle{}, e.predicates[head].sorts...)
			p.order = append(p.order, head)
			advanced = true
		}
		if pending && !advanced {
			return nil, ErrUDocDatalogFragment
		}
	}
	for _, d := range e.predicateOrder {
		if !p.productHead(d) && p.rules[d] == nil && !p.docs[d] && !p.finite[d] {
			return nil, ErrUDocDatalogFragment
		}
	}
	return p, nil
}
