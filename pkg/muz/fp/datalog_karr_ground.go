package fp

// Source: dl_compiler.cpp make_assembling_code builds no-tail heads from
// singleton and total relations, then duplicates and renames columns.
import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type karrGroundRule struct {
	// A nil value denotes a full one-column relation for an unbound variable.
	values  []*big.Rat
	head    karrHeadPlan
	unbound bool
}

func (a *karrDatalogActivation) compileGroundRule(rule *base.Rule) (ast.ASTHandle, karrGroundRule, error) {
	var none ast.ASTHandle
	var out karrGroundRule
	n, err := rule.TailSize()
	if err != nil {
		return none, out, err
	}
	if n != 0 {
		return none, out, ErrKarrDatalogFragment
	}
	head, err := rule.Head()
	if err != nil {
		return none, out, err
	}
	decl, err := a.manager.GetDecl(head)
	if err != nil {
		return none, out, err
	}
	if a.seeds[decl] == nil {
		return none, out, ErrKarrDatalogFragment
	}
	arity, err := a.manager.GetNumArgs(head)
	if err != nil {
		return none, out, err
	}
	positions := make(map[ast.ASTHandle]uint32)
	selected := make([]uint32, arity)
	for i := uint32(0); i < arity; i++ {
		arg, err := a.manager.GetArg(head, i)
		if err != nil {
			return none, out, err
		}
		kind, err := a.manager.GetKind(arg)
		if err != nil {
			return none, out, err
		}
		if kind == ast.ASTVar {
			out.unbound = true
			if column, ok := positions[arg]; ok {
				selected[i] = column
			} else {
				selected[i] = uint32(len(out.values))
				positions[arg] = selected[i]
				out.values = append(out.values, nil)
			}
			continue
		}
		value, _, matched, err := a.manager.ArithmeticNumeralText(arg)
		if err != nil {
			return none, out, err
		}
		if !matched {
			return none, out, ErrKarrDatalogFragment
		}
		num, ok := new(big.Rat).SetString(value)
		if !ok {
			return none, out, ast.ErrASTDomain
		}
		selected[i] = uint32(len(out.values))
		out.values = append(out.values, num)
	}
	used := make(map[uint32]bool)
	next := uint32(len(out.values))
	for i, column := range selected {
		if used[column] {
			out.head.duplicates = append(out.head.duplicates, column)
			selected[i] = next
			next++
		} else {
			used[column] = true
		}
	}
	visited := make([]bool, arity)
	for i := uint32(0); i < arity; i++ {
		if visited[i] {
			continue
		}
		var cycle []uint32
		for j := i; !visited[j]; j = selected[j] {
			visited[j] = true
			cycle = append(cycle, j)
		}
		if len(cycle) > 1 {
			out.head.cycles = append(out.head.cycles, cycle)
		}
	}
	return decl, out, nil
}

func (r karrGroundRule) apply(checkpoint func() bool) (*rel.KarrRelation, error) {
	if len(r.values) == 0 {
		return rel.NewKarrFull(0, checkpoint), nil
	}
	var result *rel.KarrRelation
	for _, value := range r.values {
		singleton := rel.NewKarrFull(1, checkpoint)
		if value != nil {
			singleton = rel.NewKarrEmpty(1, checkpoint)
			if err := singleton.AddFact([]*big.Rat{value}); err != nil {
				return nil, err
			}
		}
		if result == nil {
			result = singleton
			continue
		}
		var err error
		result, err = rel.KarrJoin(result, singleton, nil, nil)
		if err != nil {
			return nil, err
		}
	}
	if r.unbound && len(r.head.duplicates) > 0 && result.Dimension() > 1 {
		// make_duplicate_column projects before joining the duplicate.
		result.PrepareSourceEmptyBasis()
	}
	return r.head.apply(result)
}
