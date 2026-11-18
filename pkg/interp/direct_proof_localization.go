package interp

// Source checkpoint: src/interp/iz3proof_itp.cpp localization-variable
// creation and add_quants active branch.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

type localizationVariable struct {
	variable ast.ASTHandle
	term     ast.ASTHandle
	frame    int
}

type localizationMaps struct {
	terms  map[ast.ASTHandle]ast.ASTHandle
	proofs map[ast.ASTHandle]ast.ASTHandle
}

func (p *DirectProofInterpolator) mapsForLocalization(scope ScopeRange) (*localizationMaps, *iz3LocMaps, error) {
	contained, err := p.base.scopes.RangeContained(scope, p.scope)
	if err != nil {
		return nil, nil, err
	}
	intersects, err := p.base.scopes.RangesIntersect(scope, p.scope)
	if err != nil {
		return nil, nil, err
	}
	if !contained && intersects {
		scope = FullScopeRange()
	}
	maps := p.localizationMaps[scope]
	if maps == nil {
		maps = &localizationMaps{terms: make(map[ast.ASTHandle]ast.ASTHandle), proofs: make(map[ast.ASTHandle]ast.ASTHandle)}
		p.localizationMaps[scope] = maps
	}
	return maps, p.localizationMapRefs.lookup(scope), nil
}

// setLocalization is localization_pf_map[term] = proof (if pf) or
// localization_map[term] = value, keeping g3's map and the source's
// references together.
func (p *DirectProofInterpolator) setLocalization(maps *localizationMaps, refs *iz3LocMaps, pf bool, term, value ast.ASTHandle) error {
	manager := p.base.manager.ast
	if pf {
		maps.proofs[term] = value
		return refs.proofs.set(manager, term, value)
	}
	maps.terms[term] = value
	return refs.terms.set(manager, term, value)
}

func (p *DirectProofInterpolator) makeReflexiveInterpolation(term ast.ASTHandle) (ast.ASTHandle, error) {
	side, err := p.TermSide(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if side == TermA {
		return p.base.manager.Make(OperatorFalse)
	}
	return p.base.manager.Make(OperatorTrue)
}

func (p *DirectProofInterpolator) localizeTerm(term ast.ASTHandle, scope ScopeRange) (localized, proof ast.ASTHandle, resultErr error) {
	maps, mapRefs, err := p.mapsForLocalization(scope)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	original := term
	proof, err = p.makeReflexiveInterpolation(term)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	// localize_term calls pv->ast_scope(e), which memoizes the scope.
	if _, err = p.base.ASTScope(term); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	useCached := false
	if cached, found := maps.terms[term]; found {
		useCached = true
		sort, e := p.base.manager.Type(term)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		boolean, e := p.base.manager.IsBoolType(sort)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		if boolean {
			cachedScope, e := p.base.ASTScope(cached)
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, e
			}
			useCached, e = p.base.scopes.RangesIntersect(cachedScope, scope)
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, e
			}
		}
		if useCached {
			// A cache hit still goes through the range test below.
			// pf = localization_pf_map[e] is an operator[] lookup.
			if _, e := mapRefs.proofs.lookup(p.base.manager.ast, term); e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, e
			}
			proof = maps.proofs[term]
			term = cached
		}
	}
	if !useCached {
		args, err := p.base.manager.Args(term)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		if len(args) != 0 {
			argumentScope := scope
			op, e := p.base.manager.Operator(term)
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, e
			}
			if op == OperatorUninterpreted {
				declaration, e := p.base.manager.Symbol(term)
				if e != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, e
				}
				symbolScope, e := p.base.SymbolRange(declaration)
				if e != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, e
				}
				intersects, e := p.base.scopes.RangesIntersect(symbolScope, scope)
				if e != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, e
				}
				if intersects {
					argumentScope, e = p.base.scopes.RangeGLB(symbolScope, scope)
					if e != nil {
						return ast.ASTHandle{}, ast.ASTHandle{}, e
					}
				} else {
					argumentScope = symbolScope
				}
			} else if op == OperatorSelect {
				arrayScope, e := p.base.ASTScope(args[0])
				if e != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, e
				}
				// As in localize_term, an empty array scope also localizes the
				// arguments to that (empty) scope.
				intersects := false
				if !arrayScope.IsEmpty() {
					intersects, e = p.base.scopes.RangesIntersect(arrayScope, scope)
					if e != nil {
						return ast.ASTHandle{}, ast.ASTHandle{}, e
					}
				}
				if intersects {
					argumentScope, e = p.base.scopes.RangeGLB(arrayScope, scope)
					if e != nil {
						return ast.ASTHandle{}, ast.ASTHandle{}, e
					}
				} else {
					argumentScope = arrayScope
				}
			}
			var equalities, proofs []ast.ASTHandle
			for i, argument := range args {
				localizedArgument, argumentProof, e := p.localizeTerm(argument, argumentScope)
				if e != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, e
				}
				args[i] = localizedArgument
				argumentASTScope, e := p.base.ASTScope(localizedArgument)
				if e != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, e
				}
				argumentScope, e = p.base.scopes.RangeGLB(argumentScope, argumentASTScope)
				if e != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, e
				}
				if localizedArgument != argument {
					equality, e := p.makeEquivalence(localizedArgument, argument)
					if e != nil {
						return ast.ASTHandle{}, ast.ASTHandle{}, e
					}
					equalities = append(equalities, equality)
					proofs = append(proofs, argumentProof)
				}
			}
			term, err = p.base.manager.Clone(term, args)
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
			if len(proofs) != 0 {
				conclusion, e := p.makeEquivalence(term, original)
				if e != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, e
				}
				proof, e = p.MakeCongruences(equalities, conclusion, proofs)
				if e != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, e
				}
			}
		}
		if err = p.setLocalization(maps, mapRefs, true, original, proof); err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		if err = p.setLocalization(maps, mapRefs, false, original, term); err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
	}
	termScope, err := p.base.ASTScope(term)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	intersects, err := p.base.scopes.RangesIntersect(termScope, scope)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if intersects {
		return term, proof, nil
	}
	frame, err := p.base.scopes.RangeNear(termScope, scope)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	variable, err := p.freshLocalizationVariable(term, frame)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if err = p.setLocalization(maps, mapRefs, false, original, variable); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	equivalence, err := p.makeEquivalence(variable, term)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	assumption, err := p.MakeAssumption(frame, []ast.ASTHandle{equivalence})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	proof, err = p.MakeTransitivity(variable, term, original, assumption, proof)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if err = p.setLocalization(maps, mapRefs, true, original, proof); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if !scope.IsFull() {
		// localization_maps_per_range[range_full()], taken as is.
		full := p.localizationMaps[FullScopeRange()]
		if full == nil {
			full = &localizationMaps{terms: make(map[ast.ASTHandle]ast.ASTHandle), proofs: make(map[ast.ASTHandle]ast.ASTHandle)}
			p.localizationMaps[FullScopeRange()] = full
		}
		fullRefs := p.localizationMapRefs.lookup(FullScopeRange())
		if e := p.setLocalization(full, fullRefs, false, original, variable); e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		if e := p.setLocalization(full, fullRefs, true, original, proof); e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
	}
	return variable, proof, nil
}

// MakeAxiom localizes every literal over the full proof range before applying
// the source axiom interpolation rule.
func (p *DirectProofInterpolator) MakeAxiom(conclusion []ast.ASTHandle) (ast.ASTHandle, error) {
	return p.MakeAxiomInRange(conclusion, FullScopeRange())
}

// MakeAxiomInRange localizes an axiom clause to the requested source range and
// transports the resulting assumption through each localization equality.
func (p *DirectProofInterpolator) MakeAxiomInRange(conclusion []ast.ASTHandle, scope ScopeRange) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	localized := make([]ast.ASTHandle, len(conclusion))
	equalities := make([]ast.ASTHandle, 0, len(conclusion))
	proofs := make([]ast.ASTHandle, 0, len(conclusion))
	for i, literal := range conclusion {
		var proof ast.ASTHandle
		var err error
		localized[i], proof, err = p.localizeTerm(literal, scope)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		literalScope, err := p.base.ASTScope(localized[i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		scope, err = p.base.scopes.RangeGLB(scope, literalScope)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if localized[i] != literal {
			equality, e := p.makeEquivalence(localized[i], literal)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			equalities = append(equalities, equality)
			proofs = append(proofs, proof)
		}
	}
	interpolation, err := p.MakeAssumption(scope.Max(), localized)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for i, equality := range equalities {
		interpolation, err = p.MakeModusPonens(equality, interpolation, proofs[i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return interpolation, nil
}

func (p *DirectProofInterpolator) freshLocalizationVariable(term ast.ASTHandle, frame int) (ast.ASTHandle, error) {
	sort, err := p.base.manager.Type(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	variable, err := p.base.manager.MakeVariable(fmt.Sprintf("%%%d", len(p.localizationVariables)), sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// localization_vars.push_back(LocVar(var,term,frame)).
	variableRef, err := p.base.manager.ast.NewRef(variable)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	termRef, err := p.base.manager.ast.NewRef(term)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, p.base.manager.ast.DelRef(&variableRef))
	}
	p.localizationRefs = append(p.localizationRefs, localizationVariableRefs{variable: variableRef, term: termRef})
	declaration, err := p.base.manager.Symbol(variable)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.base.symbolRange[declaration] = FullScopeRange()
	p.localizationVariables = append(p.localizationVariables, localizationVariable{variable: variable, term: term, frame: frame})
	return variable, nil
}

func (p *DirectProofInterpolator) addLocalizationQuantifiers(expression ast.ASTHandle) (ast.ASTHandle, error) {
	result := expression
	for i := len(p.localizationVariables) - 1; i >= 0; i-- {
		variable := p.localizationVariables[i]
		inside, err := p.base.scopes.InRange(variable.frame, p.scope)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		quantifier := OperatorForall
		if inside {
			quantifier = OperatorExists
		}
		result, err = p.base.manager.ApplyQuantifier(quantifier, variable.variable, result)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return result, nil
}
