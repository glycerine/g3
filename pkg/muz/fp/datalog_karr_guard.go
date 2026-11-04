package fp

// Concrete retained portion of dl_compiler.cpp's interpreted filter path and
// karr_relation.cpp filter_interpreted: one comparison over bound integer
// variables and numerals, without the fused filter-and-project operation.
import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type karrGuardRule struct {
	body           ast.ASTHandle
	row            []big.Rat
	constant       big.Rat
	equal          bool
	secondRow      []big.Rat
	secondConstant big.Rat
	projection     []uint32
	head           karrHeadPlan
}

func (a *karrDatalogActivation) compileGuardRule(rule *base.Rule) (ast.ASTHandle, karrGuardRule, error) {
	var none ast.ASTHandle
	var out karrGuardRule
	count, err := rule.TailSize()
	if err != nil {
		return none, out, err
	}
	if count != 2 {
		return none, out, ErrKarrDatalogFragment
	}
	count, err = rule.UninterpretedTailSize()
	if err != nil {
		return none, out, err
	}
	if count != 1 {
		return none, out, ErrKarrDatalogFragment
	}
	head, err := rule.Head()
	if err != nil {
		return none, out, err
	}
	tail, neg, err := rule.Tail(0)
	if err != nil {
		return none, out, err
	}
	if neg {
		return none, out, ErrKarrDatalogFragment
	}
	guard, neg, err := rule.Tail(1)
	if err != nil {
		return none, out, err
	}
	if neg {
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
	arity, err := a.manager.GetNumArgs(tail)
	if err != nil {
		return none, out, err
	}
	positions := make(map[ast.ASTHandle]uint32, arity)
	columns := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
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
		columns[i] = arg
	}
	// The native combined filter/project removes every tail variable absent
	// from the head, sorting source column indices before projection.
	used := make(map[ast.ASTHandle]bool)
	n, err := a.manager.GetNumArgs(head)
	if err != nil {
		return none, out, err
	}
	for i := uint32(0); i < n; i++ {
		arg, err := a.manager.GetArg(head, i)
		if err != nil {
			return none, out, err
		}
		used[arg] = true
	}
	kept := make([]ast.ASTHandle, 0, len(columns))
	for i, arg := range columns {
		if used[arg] {
			kept = append(kept, arg)
		} else {
			out.projection = append(out.projection, uint32(i))
		}
	}
	family, err := a.manager.GetFamilyID(guard)
	if err != nil {
		return none, out, err
	}
	op, err := a.manager.GetDeclKind(guard)
	if err != nil {
		return none, out, err
	}
	guardArity, err := a.manager.GetNumArgs(guard)
	if err != nil {
		return none, out, err
	}
	negated := family == a.manager.BasicFamilyID() && op == ast.ASTOpNot
	if negated {
		if guardArity != 1 {
			return none, out, ErrKarrDatalogFragment
		}
		guard, err = a.manager.GetArg(guard, 0)
		if err != nil {
			return none, out, err
		}
		family, err = a.manager.GetFamilyID(guard)
		if err != nil {
			return none, out, err
		}
		op, err = a.manager.GetDeclKind(guard)
		if err != nil {
			return none, out, err
		}
		guardArity, err = a.manager.GetNumArgs(guard)
		if err != nil {
			return none, out, err
		}
	}
	if guardArity != 2 {
		return none, out, ErrKarrDatalogFragment
	}
	if !negated && family == a.manager.BasicFamilyID() && op == ast.ASTOpOr {
		if err = a.compileKarrDisjunction(guard, positions, &out); err != nil {
			return none, out, err
		}
		out.head, err = a.compileKarrHead(head, kept)
		if err != nil {
			return none, out, err
		}
		return hd, out, nil
	}
	leftMul, rightMul := int64(1), int64(-1)
	if !negated && family == a.manager.BasicFamilyID() && op == ast.ASTOpEq {
		out.equal = true
	} else {
		arith, err := a.manager.RegisterArithmeticPlugin()
		if err != nil {
			return none, out, err
		}
		if family != arith || op < 2 || op > 5 {
			return none, out, ErrKarrDatalogFragment
		}
		if negated {
			// not (<=, >=, <, >) becomes (>, <, >=, <=).
			switch op {
			case 2:
				op = 5
			case 3:
				op = 4
			case 4:
				op = 3
			case 5:
				op = 2
			}
		}
		if op == 2 || op == 4 {
			leftMul, rightMul = -1, 1
		}
		if op == 4 || op == 5 {
			out.constant.SetInt64(-1)
		}
	}
	out.row = make([]big.Rat, arity)
	arithmetic, err := a.manager.RegisterArithmeticPlugin()
	if err != nil {
		return none, out, err
	}
	integer, err := a.manager.MkFamilySort(arithmetic, 1, nil)
	if err != nil {
		return none, out, err
	}
	add := func(expr ast.ASTHandle, factor int64) error {
		return a.addKarrLinear(expr, big.NewRat(factor, 1), positions, out.row, &out.constant, integer, arithmetic)
	}
	left, err := a.manager.GetArg(guard, 0)
	if err != nil {
		return none, out, err
	}
	right, err := a.manager.GetArg(guard, 1)
	if err != nil {
		return none, out, err
	}
	if err = add(left, leftMul); err != nil {
		return none, out, err
	}
	if err = add(right, rightMul); err != nil {
		return none, out, err
	}
	out.head, err = a.compileKarrHead(head, kept)
	if err != nil {
		return none, out, err
	}
	return hd, out, nil
}

func (a *karrDatalogActivation) applyGuardRule(r karrGuardRule, source *rel.KarrRelation) (*rel.KarrRelation, error) {
	result := source.Clone()
	if err := rel.KarrSourceFilterConstraint(result, r.row, &r.constant, r.equal); err != nil {
		return nil, err
	}
	if r.secondRow != nil {
		if err := rel.KarrSourceFilterConstraint(result, r.secondRow, &r.secondConstant, false); err != nil {
			return nil, err
		}
	}
	if err := a.checkpoint(); err != nil {
		return nil, err
	}
	if len(r.projection) > 0 {
		var err error
		result, err = rel.KarrSourceProject(result, r.projection)
		if err != nil {
			return nil, err
		}
		if err = a.checkpoint(); err != nil {
			return nil, err
		}
	}
	result, err := r.head.apply(result)
	if err != nil {
		return nil, err
	}
	if err := a.checkpoint(); err != nil {
		return nil, err
	}
	return result, nil
}
