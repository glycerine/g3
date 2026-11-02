package duality

import "github.com/glycerine/g3/pkg/ast"

// proveQuantifiedHornLiteral derives the abstraction wrapper
// (= (forall (...) P(...)) true) from one transformed universally quantified
// Horn definition. It never invents an assertion: the hyper-resolution parent
// is the exact transformed source clause and all residual theory antecedents
// remain independently checked side conditions.
func (g *RPFP) proveQuantifiedHornLiteral(source *RPFP, current int, target ast.ASTHandle, refs *[]ast.ASTRef) (ast.ASTRef, error) {
	m := g.context.manager
	family, err := m.GetFamilyID(target)
	if err != nil {
		return ast.ASTRef{}, err
	}
	op, err := m.GetDeclKind(target)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if family != m.BasicFamilyID() || (op != ast.ASTOpEq && op != ast.ASTOpIff) {
		return ast.ASTRef{}, nil
	}
	left, err := m.GetArg(target, 0)
	if err != nil {
		return ast.ASTRef{}, err
	}
	right, err := m.GetArg(target, 1)
	if err != nil {
		return ast.ASTRef{}, err
	}
	// QuantifierAbstraction emits the exact (forall ...) = true orientation.
	// Keep this helper deliberately narrower than Boolean commutativity: the
	// proof produced below has precisely that fact.
	if m.IsTrue(left) || !m.IsTrue(right) {
		return ast.ASTRef{}, nil
	}
	kind, err := m.GetKind(left)
	if err != nil || kind != ast.ASTQuantifier {
		return ast.ASTRef{}, err
	}
	forall, err := m.IsForall(left)
	if err != nil || !forall {
		return ast.ASTRef{}, err
	}
	targetBody, err := m.GetQuantifierBody(left)
	if err != nil {
		return ast.ASTRef{}, err
	}
	targetDecl, err := m.GetDecl(targetBody)
	if err != nil {
		return ast.ASTRef{}, err
	}
	var definition ast.ASTHandle
	for i, ref := range source.sourceClauses {
		if i == current {
			continue
		}
		formula, err := m.RefGet(ref)
		if err != nil {
			return ast.ASTRef{}, err
		}
		bodyFormula := formula
		formulaKind, err := m.GetKind(formula)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if formulaKind == ast.ASTQuantifier {
			isForall, err := m.IsForall(formula)
			if err != nil || !isForall {
				continue
			}
			bodyFormula, err = m.GetQuantifierBody(formula)
			if err != nil {
				return ast.ASTRef{}, err
			}
		}
		formulaFamily, err := m.GetFamilyID(bodyFormula)
		if err != nil {
			return ast.ASTRef{}, err
		}
		formulaOp, err := m.GetDeclKind(bodyFormula)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if formulaFamily == m.BasicFamilyID() && formulaOp == ast.ASTOpImplies {
			head, err := m.GetArg(bodyFormula, 1)
			if err != nil {
				return ast.ASTRef{}, err
			}
			headDecl, err := m.GetDecl(head)
			if err == nil && headDecl == targetDecl && formulaKind == ast.ASTQuantifier {
				supported, err := g.structuralQuantifiedDefinition(bodyFormula)
				if err != nil {
					return ast.ASTRef{}, err
				}
				if supported {
					definition = formula
					break
				}
			}
			continue
		}
	}
	if definition.IsNull() {
		return ast.ASTRef{}, nil
	}
	definitionBody, err := m.GetQuantifierBody(definition)
	if err != nil {
		return ast.ASTRef{}, err
	}
	antecedent, err := m.GetArg(definitionBody, 0)
	if err != nil {
		return ast.ASTRef{}, err
	}
	sourceHead, err := m.GetArg(definitionBody, 1)
	if err != nil {
		return ast.ASTRef{}, err
	}
	n, err := m.GetNumDeclarations(definition)
	if err != nil {
		return ast.ASTRef{}, err
	}
	substitution := make([]ast.ASTHandle, n)
	headArgs, err := m.GetNumArgs(sourceHead)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if targetArgs, getErr := m.GetNumArgs(targetBody); getErr != nil || targetArgs != headArgs {
		return ast.ASTRef{}, getErr
	}
	for i := uint32(0); i < headArgs; i++ {
		sourceArg, err := m.GetArg(sourceHead, i)
		if err != nil {
			return ast.ASTRef{}, err
		}
		kind, err := m.GetKind(sourceArg)
		if err != nil || kind != ast.ASTVar {
			return ast.ASTRef{}, err
		}
		index, err := m.GetVarIndex(sourceArg)
		if err != nil || index >= n {
			return ast.ASTRef{}, err
		}
		targetArg, err := m.GetArg(targetBody, i)
		if err != nil {
			return ast.ASTRef{}, err
		}
		sourceSort, err := m.GetSort(sourceArg)
		if err != nil {
			return ast.ASTRef{}, err
		}
		targetSort, err := m.GetSort(targetArg)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if sourceSort != targetSort {
			return ast.ASTRef{}, nil
		}
		if !substitution[index].IsNull() && substitution[index] != targetArg {
			return ast.ASTRef{}, nil
		}
		substitution[index] = targetArg
	}
	atoms := []ast.ASTHandle{antecedent}
	antecedentFamily, err := m.GetFamilyID(antecedent)
	if err != nil {
		return ast.ASTRef{}, err
	}
	antecedentOp, err := m.GetDeclKind(antecedent)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if antecedentFamily == m.BasicFamilyID() && antecedentOp == ast.ASTOpAnd {
		count, err := m.GetNumArgs(antecedent)
		if err != nil {
			return ast.ASTRef{}, err
		}
		atoms = make([]ast.ASTHandle, count)
		for i := range atoms {
			atoms[i], err = m.GetArg(antecedent, uint32(i))
			if err != nil {
				return ast.ASTRef{}, err
			}
		}
	}
	for _, atom := range atoms {
		family, err := m.GetFamilyID(atom)
		if err != nil {
			return ast.ASTRef{}, err
		}
		op, err := m.GetDeclKind(atom)
		if err != nil || family != m.BasicFamilyID() || op != ast.ASTOpEq {
			continue
		}
		for side := uint32(0); side < 2; side++ {
			variable, err := m.GetArg(atom, side)
			if err != nil {
				return ast.ASTRef{}, err
			}
			kind, err := m.GetKind(variable)
			if err != nil || kind != ast.ASTVar {
				continue
			}
			index, err := m.GetVarIndex(variable)
			if err != nil || index >= n || !substitution[index].IsNull() {
				continue
			}
			value, err := m.GetArg(atom, 1-side)
			if err != nil {
				return ast.ASTRef{}, err
			}
			valueKind, err := m.GetKind(value)
			if err != nil || valueKind == ast.ASTVar {
				continue
			}
			free, err := m.HasFreeVars(value)
			if err != nil {
				return ast.ASTRef{}, err
			}
			if free {
				continue
			}
			variableSort, err := m.GetSort(variable)
			if err != nil {
				return ast.ASTRef{}, err
			}
			valueSort, err := m.GetSort(value)
			if err != nil {
				return ast.ASTRef{}, err
			}
			if variableSort != valueSort {
				continue
			}
			substitution[index] = value
		}
	}
	for _, value := range substitution {
		if value.IsNull() {
			return ast.ASTRef{}, nil
		}
	}
	definitionProof, err := m.MkProofAsserted(definition)
	if err != nil {
		return ast.ASTRef{}, err
	}
	definitionRef, err := m.NewRef(definitionProof)
	if err != nil {
		return ast.ASTRef{}, err
	}
	*refs = append(*refs, definitionRef)
	quantifiedProof, err := m.MkProofHyperResolve(
		[]ast.ASTHandle{definitionProof}, left, nil,
		[][]ast.ASTHandle{substitution},
	)
	if err != nil {
		return ast.ASTRef{}, err
	}
	quantifiedRef, err := m.NewRef(quantifiedProof)
	if err != nil {
		return ast.ASTRef{}, err
	}
	*refs = append(*refs, quantifiedRef)
	wrapperProof, err := m.MkProofIffTrue(quantifiedProof)
	if err != nil {
		return ast.ASTRef{}, err
	}
	wrapperFact, err := m.GetProofFact(wrapperProof)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if wrapperFact != target {
		return ast.ASTRef{}, nil
	}
	return m.NewRef(wrapperProof)
}

// structuralQuantifiedDefinition bounds the specialized constructor to the
// clause shape emitted by QuantifierAbstraction: an all-variable predicate
// head guarded only by equalities (or their conjunction). Relation antecedents
// require real parent proofs and therefore use the native fallback instead.
func (g *RPFP) structuralQuantifiedDefinition(body ast.ASTHandle) (bool, error) {
	m := g.context.manager
	head, err := m.GetArg(body, 1)
	if err != nil {
		return false, err
	}
	n, err := m.GetNumArgs(head)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < n; i++ {
		arg, err := m.GetArg(head, i)
		if err != nil {
			return false, err
		}
		kind, err := m.GetKind(arg)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTVar {
			return false, nil
		}
	}
	antecedent, err := m.GetArg(body, 0)
	if err != nil {
		return false, err
	}
	atoms := []ast.ASTHandle{antecedent}
	family, err := m.GetFamilyID(antecedent)
	if err != nil {
		return false, err
	}
	op, err := m.GetDeclKind(antecedent)
	if err != nil {
		return false, err
	}
	if family == m.BasicFamilyID() && op == ast.ASTOpAnd {
		count, err := m.GetNumArgs(antecedent)
		if err != nil {
			return false, err
		}
		atoms = make([]ast.ASTHandle, count)
		for i := range atoms {
			atoms[i], err = m.GetArg(antecedent, uint32(i))
			if err != nil {
				return false, err
			}
		}
	}
	for _, atom := range atoms {
		family, err := m.GetFamilyID(atom)
		if err != nil {
			return false, err
		}
		op, err := m.GetDeclKind(atom)
		if err != nil {
			return false, err
		}
		if family != m.BasicFamilyID() || op != ast.ASTOpEq {
			return false, nil
		}
	}
	return true, nil
}
