package qe

// Source: src/qe/qe.cpp, search_tree state and branch construction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type exprQETree struct {
	manager                       *ast.ASTManager
	parent                        *exprQETree
	formula, assignment, selected ast.ASTRef
	variables                     []ast.ASTRef
	definitions                   *DefinitionVector
	atoms                         *BoolAtomIndex
	children                      []*exprQETree
	branchIndex                   map[string]int
	simplifiers                   []exprQETreeSimplifier
	// literals is the owning quant_elim_plugin's m_literals, shared by
	// every node; nil for trees built outside a plugin.
	literals *exprQELiterals
	branches unumeral.Rational // m_num_branches
	pure     bool
}

func newExprQETree(m *ast.ASTManager, config Config, parent *exprQETree, assignment ast.ASTHandle) (result *exprQETree, err error) {
	if m == nil || assignment.IsNull() {
		return nil, ast.ErrASTHandle
	}
	result = &exprQETree{manager: m, parent: parent, pure: true, branchIndex: make(map[string]int), branches: unumeral.NewRational()}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	result.assignment, err = m.NewRef(assignment)
	if err != nil {
		return result, err
	}
	result.definitions, err = NewDefinitionVector(m, config)
	if err != nil {
		return result, err
	}
	if parent != nil {
		result.simplifiers = append(result.simplifiers, parent.simplifiers...)
		result.literals = parent.literals
	}
	return result, nil
}

func (s *exprQETree) initialize(formula ast.ASTHandle) error {
	if s == nil || s.manager == nil {
		return ast.ErrASTHandle
	}
	if s.formula == (ast.ASTRef{}) {
		ref, err := s.manager.NewRef(formula)
		if err != nil {
			return err
		}
		s.formula = ref
		return nil
	}
	return s.manager.SetRef(s.formula, formula)
}

// consumeVariables follows app_ref_vector's back-to-front transfer order.
func (s *exprQETree) consumeVariables(variables []ast.ASTHandle) error {
	if s == nil || s.manager == nil {
		return ast.ErrASTHandle
	}
	for i := len(variables) - 1; i >= 0; i-- {
		ref, err := s.manager.NewRef(variables[i])
		if err != nil {
			return err
		}
		s.variables = append(s.variables, ref)
	}
	return nil
}
func (s *exprQETree) variableIndex(variable ast.ASTHandle) (int, error) {
	for i, ref := range s.variables {
		h, err := s.manager.RefGet(ref)
		if err != nil {
			return -1, err
		}
		if h == variable {
			return i, nil
		}
	}
	return -1, nil
}

// selectVariable is set_var(x, num_branches); branches is a rational of the
// manager's NumeralService().Manager.
func (s *exprQETree) selectVariable(variable ast.ASTHandle, branches *unumeral.Rational) error {
	if s == nil || s.manager == nil || branches == nil || s.selected != (ast.ASTRef{}) {
		return ast.ErrASTDomain
	}
	q := s.manager.NumeralService().Manager
	if negative, err := q.IsNeg(branches); err != nil || negative {
		return errors.Join(err, ast.ErrASTDomain)
	}
	i, err := s.variableIndex(variable)
	if err != nil {
		return err
	}
	if i < 0 {
		return ast.ErrASTDomain
	}
	s.selected = s.variables[i]
	copy(s.variables[i:], s.variables[i+1:])
	s.variables = s.variables[:len(s.variables)-1]
	return q.Copy(&s.branches, branches)
}
func (s *exprQETree) deleteVariable(variable ast.ASTHandle) error {
	if s == nil || s.manager == nil || len(s.children) != 0 {
		return ast.ErrASTDomain
	}
	i, err := s.variableIndex(variable)
	if err != nil {
		return err
	}
	if i < 0 {
		return ast.ErrASTDomain
	}
	if err = s.manager.DelRef(&s.variables[i]); err != nil {
		return err
	}
	copy(s.variables[i:], s.variables[i+1:])
	s.variables = s.variables[:len(s.variables)-1]
	s.pure = false
	return nil
}
func (s *exprQETree) isPure() bool {
	for node := s; node != nil; node = node.parent {
		if !node.pure {
			return false
		}
	}
	return true
}
func (s *exprQETree) isUnit() bool { return len(s.children) == 1 && len(s.branchIndex) == 0 }

func (s *exprQETree) newChild(config Config, assignment ast.ASTHandle) (child *exprQETree, err error) {
	child, err = newExprQETree(s.manager, config, s, assignment)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, child.Close())
			child = nil
		}
	}()
	for _, ref := range s.variables {
		copy, e := s.manager.CopyRef(ref)
		if e != nil {
			return child, e
		}
		child.variables = append(child.variables, copy)
	}
	return child, nil
}
func (s *exprQETree) addUnit(config Config, formula ast.ASTHandle) (*exprQETree, error) {
	if s == nil || s.manager == nil || len(s.children) != 0 || len(s.branchIndex) != 0 {
		return nil, ast.ErrASTDomain
	}
	truth, err := s.manager.MkTrue()
	if err != nil {
		return nil, err
	}
	child, err := s.newChild(config, truth)
	if err != nil {
		return nil, err
	}
	if err = child.initialize(formula); err != nil {
		return nil, errors.Join(err, child.Close())
	}
	if err = s.manager.NumeralService().Manager.SetInt32(&s.branches, 1); err != nil {
		return nil, errors.Join(err, child.Close())
	}
	s.children = append(s.children, child)
	return child, nil
}

// addBranch is add_child(branch_id, assignment); id is a rational of the
// manager's NumeralService().Manager.
func (s *exprQETree) addBranch(config Config, id *unumeral.Rational, assignment ast.ASTHandle) (*exprQETree, error) {
	if s == nil || s.manager == nil || id == nil || s.selected == (ast.ASTRef{}) {
		return nil, ast.ErrASTDomain
	}
	q := s.manager.NumeralService().Manager
	if negative, err := q.IsNeg(id); err != nil || negative {
		return nil, errors.Join(err, ast.ErrASTDomain)
	}
	if c, err := q.Compare(id, &s.branches); err != nil || c >= 0 {
		return nil, errors.Join(err, ast.ErrASTDomain)
	}
	key, err := q.String(id)
	if err != nil {
		return nil, err
	}
	if _, exists := s.branchIndex[key]; exists {
		return nil, ast.ErrASTDomain
	}
	child, err := s.newChild(config, assignment)
	if err != nil {
		return nil, err
	}
	s.branchIndex[key] = len(s.children)
	s.children = append(s.children, child)
	return child, nil
}

// leaves returns owned successful leaf formulas in the source depth-first order.
func (s *exprQETree) leaves() (result []ast.ASTRef, err error) {
	if s == nil || s.manager == nil || s.parent != nil {
		return nil, ast.ErrASTHandle
	}
	defer func() {
		if err != nil {
			for i := range result {
				err = errors.Join(err, s.manager.DelRef(&result[i]))
			}
			result = nil
		}
	}()
	todo := []*exprQETree{s}
	for len(todo) > 0 {
		i := len(todo) - 1
		node := todo[i]
		todo = todo[:i]
		if len(node.children) == 0 && node.formula != (ast.ASTRef{}) && len(node.variables) == 0 && node.selected == (ast.ASTRef{}) {
			ref, e := s.manager.CopyRef(node.formula)
			if e != nil {
				return result, e
			}
			result = append(result, ref)
		}
		todo = append(todo, node.children...)
	}
	return result, nil
}

func (s *exprQETree) Close() error {
	if s == nil || s.manager == nil {
		return nil
	}
	for len(s.children) > 0 {
		i := len(s.children) - 1
		if err := s.children[i].Close(); err != nil {
			return err
		}
		s.children = s.children[:i]
	}
	if s.atoms != nil {
		if err := s.atoms.Close(); err != nil {
			return err
		}
		s.atoms = nil
	}
	if s.definitions != nil {
		if err := s.definitions.Close(); err != nil {
			return err
		}
		s.definitions = nil
	}
	for len(s.variables) > 0 {
		i := len(s.variables) - 1
		if err := s.manager.DelRef(&s.variables[i]); err != nil {
			return err
		}
		s.variables = s.variables[:i]
	}
	for _, ref := range []*ast.ASTRef{&s.selected, &s.formula, &s.assignment} {
		if *ref != (ast.ASTRef{}) {
			if err := s.manager.DelRef(ref); err != nil {
				return err
			}
		}
	}
	if err := s.manager.NumeralService().Manager.Del(&s.branches); err != nil {
		return err
	}
	s.manager = nil
	return nil
}
