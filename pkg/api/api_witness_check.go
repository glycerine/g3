package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (p *witnessInstancePlan) assertIn(parent, m *ast.ASTManager, child *APISimpleSolver) (complete bool, err error) {
	tr, err := ast.NewASTTranslation(parent, m, true)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, tr.Close()) }()
	s := witnessClauseSet{
		m:         m,
		goalSorts: make(map[ast.ASTHandle]bool, len(p.sorts)),
		terms:     make(map[ast.ASTHandle][]ast.ASTHandle),
		apps:      make(map[ast.ASTHandle][][]ast.ASTHandle),
		keys:      make(map[witnessAtomKey]*witnessAtom),
	}
	defer func() { _, e := m.ShrinkHeldRefs(s.held, 0); err = errors.Join(err, e) }()
	s.sub, err = rewriter.NewVarSubstitution(m, true)
	if s.sub != nil {
		defer func() { err = errors.Join(err, s.sub.Close()) }()
	}
	if err != nil {
		return false, err
	}
	s.out, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&s.out)) }()
	if ok, err := s.acceptsInput(parent, p.assertions); err != nil || !ok {
		return false, err
	}
	for _, sort := range p.sorts {
		st := s.keep(tr.Translate(sort))
		if s.err != nil {
			return false, s.failure()
		}
		s.goalSorts[st] = true
	}
	input := make([]ast.ASTHandle, len(p.assertions))
	for i, h := range p.assertions {
		if !s.charge(1) {
			return false, s.failure()
		}
		input[i] = s.keep(tr.Translate(h))
		if s.err != nil {
			return false, s.failure()
		}
	}
	// Translate the entire input before generating witnesses so even later
	// premises participate in the fresh declaration check.
	base := s.lowerAssertions(input)
	for _, root := range base {
		s.collectForced(root, witnessPositive)
	}
	s.generate(p.width, p.rounds)
	if s.err != nil {
		return false, s.failure()
	}
	params, err := (&SMTContext{smtContextState: child.owner}).NewParameterSet()
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, params.Close()) }()
	if err = params.SetUInt("smt.relevancy", 0); err != nil {
		return false, err
	}
	if err = child.SetParameters(params); err != nil {
		return false, err
	}
	for _, root := range append(base, s.roots...) {
		if !s.charge(1) {
			return false, s.failure()
		}
		ground, err := m.IsGround(root)
		if err != nil || !ground {
			return false, err
		}
		if err = child.Assert(root); err != nil {
			return false, err
		}
	}
	return true, nil
}

// The last assertion is the query. Visit it first so that later matching can
// prefer its witnesses, then preserve the source order of all premises.
func (s *witnessClauseSet) lowerAssertions(input []ast.ASTHandle) []ast.ASTHandle {
	type entry struct {
		h    ast.ASTHandle
		goal bool
	}
	stack := make([]entry, 0, len(input))
	for i := len(input) - 2; i >= 0; i-- {
		stack = append(stack, entry{h: input[i]})
	}
	stack = append(stack, entry{h: input[len(input)-1], goal: true})
	seen := make(map[ast.ASTHandle]bool)
	var roots []ast.ASTHandle
	for len(stack) > 0 && s.charge(1) {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[cur.h] {
			continue
		}
		seen[cur.h] = true
		hd, e := s.m.NodeHeader(cur.h)
		if e != nil {
			s.err = e
			break
		}
		if hd.Kind == ast.ASTApp && hd.Family == s.m.BasicFamilyID() && hd.DeclKind == ast.ASTOpAnd {
			if hd.NumArgs > 8192 || len(stack)+int(hd.NumArgs) > 16_384 {
				s.err = errWitnessLimit
				break
			}
			for i := hd.NumArgs; i > 0; i-- {
				h, e := s.m.GetArg(cur.h, i-1)
				if e != nil {
					s.err = e
					break
				}
				stack = append(stack, entry{h, cur.goal})
			}
		} else {
			roots = append(roots, s.lower(cur.h, witnessPositive, cur.goal))
		}
	}
	return roots
}

func (s *witnessClauseSet) failure() error {
	if errors.Is(s.err, errWitnessLimit) {
		return nil
	}
	return s.err
}
