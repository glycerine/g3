package fp

// Source: dl_compiler.cpp unary tail projection and head renaming, executed
// using karr_relation.cpp's concrete project and rename operations.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type karrUnaryRule struct {
	body    ast.ASTHandle
	removed []uint32
	cycles  [][]uint32
}

func (r karrUnaryRule) apply(source *rel.KarrRelation) (*rel.KarrRelation, error) {
	result := source
	var err error
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
	// Keep the initial route safe: every tail argument is a different variable,
	// and every head argument is a different one of those variables.
	positions := make(map[ast.ASTHandle]uint32, tailArity)
	for i := uint32(0); i < tailArity; i++ {
		arg, err := a.manager.GetArg(tail, i)
		if err != nil {
			return none, out, err
		}
		kind, err := a.manager.GetKind(arg)
		if err != nil {
			return none, out, err
		}
		if kind != ast.ASTVar {
			return none, out, ErrKarrDatalogFragment
		}
		if _, exists := positions[arg]; exists {
			return none, out, ErrKarrDatalogFragment
		}
		positions[arg] = i
	}
	selected := make([]uint32, headArity)
	used := make([]bool, tailArity)
	for i := uint32(0); i < headArity; i++ {
		arg, err := a.manager.GetArg(head, i)
		if err != nil {
			return none, out, err
		}
		position, found := positions[arg]
		if !found || used[position] {
			return none, out, ErrKarrDatalogFragment
		}
		used[position] = true
		selected[i] = position
	}
	// The source projection preserves tail order; identify each head column's
	// index in that projected relation, then decompose its permutation in cycles.
	projected := make([]uint32, tailArity)
	next := uint32(0)
	for i := uint32(0); i < tailArity; i++ {
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
