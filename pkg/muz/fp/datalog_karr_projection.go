package fp

// Source: dl_compiler.cpp unary tail selection, identical filters, projection
// and head renaming, executed using the concrete Karr relation operations.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type karrUnaryRule struct {
	body      ast.ASTHandle
	constants []uint32
	identical [][]uint32
	removed   []uint32
	cycles    [][]uint32
}

func (r karrUnaryRule) apply(source *rel.KarrRelation) (*rel.KarrRelation, error) {
	result := source
	var err error
	// The unary compiler selects and removes each constant immediately. The
	// column index therefore refers to the current, already shortened relation.
	if len(r.constants) != 0 || len(r.identical) != 0 {
		result = source.Clone()
	}
	for _, column := range r.constants {
		if !result.Empty() {
			if err = result.FilterEqual(column, true); err != nil {
				return nil, err
			}
		}
		result, err = result.Project([]uint32{column})
		if err != nil {
			return nil, err
		}
	}
	if !result.Empty() {
		for _, group := range r.identical {
			if err = result.FilterIdentical(group); err != nil {
				return nil, err
			}
		}
	}
	if len(r.removed) != 0 {
		result, err = result.Project(r.removed)
		if err != nil {
			return nil, err
		}
	}
	for _, cycle := range r.cycles {
		result, err = result.Rename(cycle)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (a *karrDatalogActivation) compileUnaryRule(rule *base.Rule) (ast.ASTHandle, karrUnaryRule, error) {
	var none ast.ASTHandle
	var out karrUnaryRule
	n, err := rule.TailSize()
	if err != nil {
		return none, out, err
	}
	if n != 1 {
		return none, out, ErrKarrDatalogFragment
	}
	n, err = rule.UninterpretedTailSize()
	if err != nil {
		return none, out, err
	}
	if n != 1 {
		return none, out, ErrKarrDatalogFragment
	}
	head, err := rule.Head()
	if err != nil {
		return none, out, err
	}
	tail, negated, err := rule.Tail(0)
	if err != nil {
		return none, out, err
	}
	if negated {
		return none, out, ErrKarrDatalogFragment
	}
	hd, err := a.manager.GetDecl(head)
	if err != nil {
		return none, out, err
	}
	out.body, err = a.manager.GetDecl(tail)
	if err != nil {
		return none, out, err
	}
	if a.seeds[hd] == nil || a.seeds[out.body] == nil {
		return none, out, ErrKarrDatalogFragment
	}
	headArity, err := a.manager.GetNumArgs(head)
	if err != nil {
		return none, out, err
	}
	tailArity, err := a.manager.GetNumArgs(tail)
	if err != nil {
		return none, out, err
	}
	if headArity > tailArity {
		return none, out, ErrKarrDatalogFragment
	}
	// Constants are selected and projected in left-to-right order. Repeated
	// variable columns refer to the shortened signature after those removals.
	positions := make(map[ast.ASTHandle][]uint32, tailArity)
	order := make([]ast.ASTHandle, 0, tailArity)
	var column uint32
	for i := uint32(0); i < tailArity; i++ {
		arg, err := a.manager.GetArg(tail, i)
		if err != nil {
			return none, out, err
		}
		kind, err := a.manager.GetKind(arg)
		if err != nil {
			return none, out, err
		}
		if kind == ast.ASTVar {
			if len(positions[arg]) == 0 {
				order = append(order, arg)
			}
			positions[arg] = append(positions[arg], column)
			column++
			continue
		}
		_, _, matched, err := a.manager.ArithmeticNumeralText(arg)
		if err != nil {
			return none, out, err
		}
		if !matched {
			return none, out, ErrKarrDatalogFragment
		}
		out.constants = append(out.constants, column)
	}
	for _, arg := range order {
		if len(positions[arg]) > 1 {
			out.identical = append(out.identical, positions[arg])
		}
	}
	if headArity > column {
		return none, out, ErrKarrDatalogFragment
	}
	selected := make([]uint32, headArity)
	used := make([]bool, column)
	headVars := make(map[ast.ASTHandle]bool, headArity)
	for i := uint32(0); i < headArity; i++ {
		arg, err := a.manager.GetArg(head, i)
		if err != nil {
			return none, out, err
		}
		indexes := positions[arg]
		if len(indexes) == 0 || headVars[arg] {
			return none, out, ErrKarrDatalogFragment
		}
		headVars[arg] = true
		position := indexes[len(indexes)-1]
		used[position] = true
		selected[i] = position
	}
	// The source projection preserves tail order; identify each head column's
	// index in that projected relation, then decompose its permutation in cycles.
	projected := make([]uint32, column)
	next := uint32(0)
	for i := uint32(0); i < column; i++ {
		if !used[i] {
			out.removed = append(out.removed, i)
		} else {
			projected[i] = next
			next++
		}
	}
	permutation := make([]uint32, headArity)
	for i, position := range selected {
		permutation[i] = projected[position]
	}
	visited := make([]bool, headArity)
	for i := uint32(0); i < headArity; i++ {
		if visited[i] {
			continue
		}
		var cycle []uint32
		for j := i; !visited[j]; j = permutation[j] {
			visited[j] = true
			cycle = append(cycle, j)
		}
		if len(cycle) > 1 {
			out.cycles = append(out.cycles, cycle)
		}
	}
	return hd, out, nil
}
