package smtcore

// Source: src/smt/smt_solver.cpp unsat-core pattern extension.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtDeclSet map[ast.ASTHandle]struct{}

func smtDeclSetsIntersect(left, right smtDeclSet) bool {
	for declaration := range left {
		if _, found := right[declaration]; found {
			return true
		}
	}
	return false
}

func (s *SMTSolver) collectUninterpretedDecls(root ast.ASTHandle, declarations smtDeclSet) error {
	visited := make(map[ast.ASTHandle]struct{})
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		expression := stack[last]
		stack = stack[:last]
		if _, found := visited[expression]; found {
			continue
		}
		visited[expression] = struct{}{}
		kind, err := s.manager.GetKind(expression)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			declaration, err := s.manager.GetDecl(expression)
			if err != nil {
				return err
			}
			family, err := s.manager.GetFamilyID(declaration)
			if err != nil {
				return err
			}
			if family == ast.NullFamilyID {
				declarations[declaration] = struct{}{}
			}
			count, err := s.manager.GetNumArgs(expression)
			if err != nil {
				return err
			}
			for i := uint32(0); i < count; i++ {
				argument, err := s.manager.GetArg(expression, i)
				if err != nil {
					return err
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			// quick_for_each_expr visits a quantifier's body, patterns and
			// no-patterns (IgnorePatterns == false).
			body, err := s.manager.GetQuantifierBody(expression)
			if err != nil {
				return err
			}
			stack = append(stack, body)
			patterns, err := s.manager.GetNumPatterns(expression)
			if err != nil {
				return err
			}
			for i := uint32(0); i < patterns; i++ {
				pattern, err := s.manager.GetPattern(expression, i)
				if err != nil {
					return err
				}
				stack = append(stack, pattern)
			}
			noPatterns, err := s.manager.GetNumNoPatterns(expression)
			if err != nil {
				return err
			}
			for i := uint32(0); i < noPatterns; i++ {
				pattern, err := s.manager.GetNoPattern(expression, i)
				if err != nil {
					return err
				}
				stack = append(stack, pattern)
			}
		case ast.ASTVar:
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (s *SMTSolver) collectPatternDecls(root ast.ASTHandle, declarations smtDeclSet) error {
	visited := make(map[ast.ASTHandle]struct{})
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		expression := stack[last]
		stack = stack[:last]
		if _, found := visited[expression]; found {
			continue
		}
		visited[expression] = struct{}{}
		kind, err := s.manager.GetKind(expression)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			count, err := s.manager.GetNumArgs(expression)
			if err != nil {
				return err
			}
			for i := uint32(0); i < count; i++ {
				argument, err := s.manager.GetArg(expression, i)
				if err != nil {
					return err
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			patterns, err := s.manager.GetNumPatterns(expression)
			if err != nil {
				return err
			}
			for i := uint32(0); i < patterns; i++ {
				pattern, err := s.manager.GetPattern(expression, i)
				if err != nil {
					return err
				}
				if err = s.collectUninterpretedDecls(pattern, declarations); err != nil {
					return err
				}
			}
			noPatterns, err := s.manager.GetNumNoPatterns(expression)
			if err != nil {
				return err
			}
			for i := uint32(0); i < noPatterns; i++ {
				pattern, err := s.manager.GetNoPattern(expression, i)
				if err != nil {
					return err
				}
				if err = s.collectUninterpretedDecls(pattern, declarations); err != nil {
					return err
				}
			}
			body, err := s.manager.GetQuantifierBody(expression)
			if err != nil {
				return err
			}
			stack = append(stack, body)
		case ast.ASTVar:
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (s *SMTSolver) collectQuantifierBodyDecls(root ast.ASTHandle, declarations smtDeclSet) error {
	visited := make(map[ast.ASTHandle]struct{})
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		expression := stack[last]
		stack = stack[:last]
		if _, found := visited[expression]; found {
			continue
		}
		visited[expression] = struct{}{}
		kind, err := s.manager.GetKind(expression)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			count, err := s.manager.GetNumArgs(expression)
			if err != nil {
				return err
			}
			for i := uint32(0); i < count; i++ {
				argument, err := s.manager.GetArg(expression, i)
				if err != nil {
					return err
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			body, err := s.manager.GetQuantifierBody(expression)
			if err != nil {
				return err
			}
			if err = s.collectUninterpretedDecls(body, declarations); err != nil {
				return err
			}
			stack = append(stack, body)
		case ast.ASTVar:
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

// name2assertion lists m_name2assertion in obj_map (table) order.
func (s *SMTSolver) name2assertion() []ast.ObjMapEntry {
	if s.name2assertionMap == nil {
		return nil
	}
	return s.name2assertionMap.Entries()
}

func (s *SMTSolver) trackedFormula(tag ast.ASTHandle) (ast.ASTHandle, bool, error) {
	for _, assertion := range s.tracked {
		if assertion.tag != tag {
			continue
		}
		formula, err := s.manager.RefGet(assertion.formula)
		return formula, true, err
	}
	return ast.ASTHandle{}, false, nil
}

func (s *SMTSolver) addPatternLiteralsToCore(core []ast.ASTHandle, maxDistance uint32) ([]ast.ASTHandle, error) {
	result := append([]ast.ASTHandle(nil), core...)
	inCore := make(map[ast.ASTHandle]struct{}, len(result))
	for _, tag := range result {
		inCore[tag] = struct{}{}
	}
	patternDecls := make(smtDeclSet)
	for distance := uint32(0); distance < maxDistance; distance++ {
		for _, tag := range result {
			formula, found, err := s.trackedFormula(tag)
			if err != nil {
				return nil, err
			}
			if found {
				if err = s.collectPatternDecls(formula, patternDecls); err != nil {
					return nil, err
				}
			}
		}
		added := false
		for _, entry := range s.name2assertion() {
			tag, formula := entry.Key, entry.Value[0]
			if _, found := inCore[tag]; found {
				continue
			}
			assertionDecls := make(smtDeclSet)
			if err := s.collectUninterpretedDecls(formula, assertionDecls); err != nil {
				return nil, err
			}
			if smtDeclSetsIntersect(patternDecls, assertionDecls) {
				result = append(result, tag)
				inCore[tag] = struct{}{}
				added = true
			}
		}
		if !added {
			break
		}
	}
	return result, nil
}

func (s *SMTSolver) addNonlocalPatternLiteralsToCore(core []ast.ASTHandle) ([]ast.ASTHandle, error) {
	result := append([]ast.ASTHandle(nil), core...)
	inCore := make(map[ast.ASTHandle]struct{}, len(result))
	for _, tag := range result {
		inCore[tag] = struct{}{}
	}
	for _, entry := range s.name2assertion() {
		tag, formula := entry.Key, entry.Value[0]
		if _, found := inCore[tag]; found {
			continue
		}
		patternDecls := make(smtDeclSet)
		if err := s.collectPatternDecls(formula, patternDecls); err != nil {
			return nil, err
		}
		bodyDecls := make(smtDeclSet)
		if err := s.collectQuantifierBodyDecls(formula, bodyDecls); err != nil {
			return nil, err
		}
		for declaration := range patternDecls {
			if _, local := bodyDecls[declaration]; !local {
				result = append(result, tag)
				inCore[tag] = struct{}{}
				break
			}
		}
	}
	return result, nil
}

// updateCoreExtendParams is the smt_params_helper part of
// smt_solver::updt_params: each getter falls back to the global smt module.
func (s *SMTSolver) updateCoreExtendParams() (resultErr error) {
	global, err := s.kernel.paramGlobals.SourceGetModuleName("smt")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, s.config.Params.Del(&global)) }()
	name := "core.extend_patterns"
	extend, err := s.config.Params.GetBoolFallbackName(&s.params, &name, &global, false)
	if err != nil {
		return err
	}
	name = "core.extend_patterns.max_distance"
	maxDistance, err := s.config.Params.GetUIntFallbackName(&s.params, &name, &global, ^uint32(0))
	if err != nil {
		return err
	}
	name = "core.extend_nonlocal_patterns"
	nonlocal, err := s.config.Params.GetBoolFallbackName(&s.params, &name, &global, false)
	if err != nil {
		return err
	}
	s.coreExtendPatterns = extend
	s.coreExtendPatternsMaxDistance = maxDistance
	s.coreExtendNonlocalPatterns = nonlocal
	return nil
}

// extendUnsatCore is the tail of smt_solver::get_unsat_core.
func (s *SMTSolver) extendUnsatCore(core []ast.ASTHandle) ([]ast.ASTHandle, error) {
	var err error
	if s.coreExtendPatterns {
		core, err = s.addPatternLiteralsToCore(core, s.coreExtendPatternsMaxDistance)
		if err != nil {
			return nil, err
		}
	}
	if s.coreExtendNonlocalPatterns {
		return s.addNonlocalPatternLiteralsToCore(core)
	}
	return core, nil
}
