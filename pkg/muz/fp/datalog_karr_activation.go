package fp

// Concrete acyclic identity-rule consumer of src/muz/rel/karr_relation.cpp.
// It intentionally rejects shapes requiring the general relational compiler.
import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrKarrDatalogFragment = errors.New("muz/fp: unsupported Karr Datalog fragment")

type karrDatalogActivation struct {
	manager *ast.ASTManager
	rules   *base.RuleSet
	seeds   map[ast.ASTHandle]*rel.KarrRelation
	results map[ast.ASTHandle]*rel.KarrRelation
	closed  bool
}

func newKarrDatalogActivation(manager *ast.ASTManager, rules *base.RuleSet, declarations []ast.ASTHandle) (*karrDatalogActivation, error) {
	if manager == nil || rules == nil {
		return nil, ErrKarrDatalogFragment
	}
	a := &karrDatalogActivation{manager: manager, rules: rules, seeds: make(map[ast.ASTHandle]*rel.KarrRelation)}
	if err := a.syncPredicates(declarations); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *karrDatalogActivation) checkpoint() error {
	canceled, err := a.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return z3exception.NewDefaultException(a.manager.ResourceLimit().CancelMessage())
	}
	return nil
}

func (a *karrDatalogActivation) syncPredicates(declarations []ast.ASTHandle) error {
	if a == nil || a.closed {
		return ErrKarrDatalogFragment
	}
	seen := make(map[ast.ASTHandle]bool, len(declarations))
	// The initial typed path admits only integer columns and nullaries.
	family, err := a.manager.RegisterArithmeticPlugin()
	if err != nil {
		return err
	}
	integer, err := a.manager.MkFamilySort(family, 1, nil)
	if err != nil {
		return err
	}
	boolean, err := a.manager.MkBoolSort()
	if err != nil {
		return err
	}
	pending := make(map[ast.ASTHandle]*rel.KarrRelation)
	for _, d := range declarations {
		if seen[d] {
			return ErrKarrDatalogFragment
		}
		seen[d] = true
		sort, err := a.manager.GetRange(d)
		if err != nil {
			return err
		}
		if sort != boolean {
			return ErrKarrDatalogFragment
		}
		arity, err := a.manager.GetArity(d)
		if err != nil {
			return err
		}
		for i := uint32(0); i < arity; i++ {
			s, err := a.manager.GetDomain(d, i)
			if err != nil {
				return err
			}
			if s != integer {
				return ErrKarrDatalogFragment
			}
		}
		if a.seeds[d] == nil {
			pending[d] = rel.NewKarrEmpty(arity, func() bool { return a.checkpoint() == nil })
		}
	}
	for d := range a.seeds {
		if !seen[d] {
			return ErrKarrDatalogFragment
		}
	}
	for d, r := range pending {
		a.seeds[d] = r
	}
	return nil
}

func (a *karrDatalogActivation) addFact(d ast.ASTHandle, values []ast.ASTHandle) error {
	if a == nil || a.closed || a.seeds[d] == nil {
		return ErrKarrDatalogFragment
	}
	if !a.seeds[d].Empty() {
		return ErrKarrDatalogFragment
	} // source add_fact asserts empty
	nums := make([]*big.Rat, len(values))
	for i, v := range values {
		text, _, matched, err := a.manager.ArithmeticNumeralText(v)
		if err != nil {
			return err
		}
		if matched {
			n, ok := new(big.Rat).SetString(text)
			if !ok {
				return ast.ErrASTDomain
			}
			nums[i] = n
		}
	}
	candidate := a.seeds[d].Clone()
	if err := candidate.AddFact(nums); err != nil {
		return err
	}
	a.seeds[d] = candidate
	a.results = nil
	return nil
}

func (a *karrDatalogActivation) clone(rules *base.RuleSet) (*karrDatalogActivation, error) {
	if a == nil || a.closed || rules == nil {
		return nil, ErrKarrDatalogFragment
	}
	b := &karrDatalogActivation{manager: a.manager, rules: rules, seeds: make(map[ast.ASTHandle]*rel.KarrRelation, len(a.seeds))}
	for d, r := range a.seeds {
		b.seeds[d] = r.Clone()
	}
	return b, nil
}

func (a *karrDatalogActivation) close() error {
	if a != nil {
		a.closed = true
		a.seeds = nil
		a.results = nil
	}
	return nil
}
func (a *karrDatalogActivation) reset() error {
	if a == nil || a.closed {
		return ErrKarrDatalogFragment
	}
	for d, r := range a.seeds {
		a.seeds[d] = rel.NewKarrEmpty(r.Dimension(), func() bool { return a.checkpoint() == nil })
	}
	a.results = nil
	return nil
}

// query uses a dependency walk to evaluate source identity-copy rules in
// topological order, publishing a snapshot only on success.
func (a *karrDatalogActivation) query(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if a == nil || a.closed {
		return lbool.Undef, ErrKarrDatalogFragment
	}
	a.results = nil
	if err := a.checkpoint(); err != nil {
		return lbool.Undef, err
	}
	edges := make(map[ast.ASTHandle][]ast.ASTHandle)
	count, err := a.rules.Len()
	if err != nil {
		return lbool.Undef, err
	}
	for i := 0; i < count; i++ {
		rule, err := a.rules.Rule(i)
		if err != nil {
			return lbool.Undef, err
		}
		head, body, err := a.identityRule(rule)
		err = errors.Join(err, rule.Close())
		if err != nil {
			return lbool.Undef, err
		}
		edges[head] = append(edges[head], body)
	}
	snapshot := make(map[ast.ASTHandle]*rel.KarrRelation, len(a.seeds))
	for d, r := range a.seeds {
		snapshot[d] = r.Clone()
	}
	marks := make(map[ast.ASTHandle]uint8)
	var visit func(ast.ASTHandle) error
	visit = func(d ast.ASTHandle) error {
		if err := a.checkpoint(); err != nil {
			return err
		}
		if marks[d] == 1 {
			return ErrKarrDatalogFragment
		}
		if marks[d] == 2 {
			return nil
		}
		target := snapshot[d]
		if target == nil {
			return ErrKarrDatalogFragment
		}
		marks[d] = 1
		for _, src := range edges[d] {
			if err := visit(src); err != nil {
				return err
			}
			if _, err := target.Union(snapshot[src]); err != nil {
				return err
			}
			if err := a.checkpoint(); err != nil {
				return err
			}
		}
		marks[d] = 2
		return nil
	}
	for d := range snapshot {
		if err := visit(d); err != nil {
			return lbool.Undef, err
		}
	}
	status := lbool.False
	if len(declarations) == 0 {
		status = lbool.True
	}
	for _, d := range declarations {
		r := snapshot[d]
		if r == nil {
			return lbool.Undef, ErrKarrDatalogFragment
		}
		if !r.Empty() {
			status = lbool.Undef
		}
	}
	if err := a.checkpoint(); err != nil {
		return lbool.Undef, err
	}
	a.results = snapshot
	return status, nil
}

func (a *karrDatalogActivation) identityRule(rule *base.Rule) (ast.ASTHandle, ast.ASTHandle, error) {
	var none ast.ASTHandle
	n, err := rule.TailSize()
	if err != nil {
		return none, none, err
	}
	if n != 1 {
		return none, none, ErrKarrDatalogFragment
	}
	n, err = rule.UninterpretedTailSize()
	if err != nil {
		return none, none, err
	}
	if n != 1 {
		return none, none, ErrKarrDatalogFragment
	}
	h, err := rule.Head()
	if err != nil {
		return none, none, err
	}
	b, neg, err := rule.Tail(0)
	if err != nil {
		return none, none, err
	}
	if neg {
		return none, none, ErrKarrDatalogFragment
	}
	hd, err := a.manager.GetDecl(h)
	if err != nil {
		return none, none, err
	}
	bd, err := a.manager.GetDecl(b)
	if err != nil {
		return none, none, err
	}
	if a.seeds[hd] == nil || a.seeds[bd] == nil {
		return none, none, ErrKarrDatalogFragment
	}
	arity, err := a.manager.GetNumArgs(h)
	if err != nil {
		return none, none, err
	}
	other, err := a.manager.GetNumArgs(b)
	if err != nil {
		return none, none, err
	}
	if arity != other {
		return none, none, ErrKarrDatalogFragment
	}
	seen := make(map[uint32]bool)
	for i := uint32(0); i < arity; i++ {
		x, err := a.manager.GetArg(h, i)
		if err != nil {
			return none, none, err
		}
		y, err := a.manager.GetArg(b, i)
		if err != nil {
			return none, none, err
		}
		if x != y {
			return none, none, ErrKarrDatalogFragment
		}
		kind, err := a.manager.GetKind(x)
		if err != nil {
			return none, none, err
		}
		if kind != ast.ASTVar {
			return none, none, ErrKarrDatalogFragment
		}
		index, err := a.manager.GetVarIndex(x)
		if err != nil {
			return none, none, err
		}
		if seen[index] {
			return none, none, ErrKarrDatalogFragment
		}
		seen[index] = true
	}
	return hd, bd, nil
}
