package spacer

// Source: src/muz/spacer/spacer_util.cpp compute_implicant_literals and
// implicant_picker.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// computeImplicantLiterals follows Spacer's source picker: select one
// model-true path through Boolean structure and retain model-true theory
// literals without running the separate model-cone minimizer.
func computeImplicantLiterals(manager *ast.ASTManager, md *model.ModelCore, config model.ModelEvaluatorConfig, formulas []ast.ASTHandle) (_ []ast.ASTRef, resultErr error) {
	if manager == nil || md == nil || md.DerivedModel() == nil {
		return nil, ast.ErrASTHandle
	}
	evaluator, err := model.NewModelEvaluator(md.DerivedModel().Core(), config)
	if err != nil {
		return nil, err
	}
	if err = evaluator.SetModelCompletion(false); err != nil {
		return nil, errors.Join(err, evaluator.Close())
	}
	var result []ast.ASTRef
	defer func() {
		resultErr = errors.Join(resultErr, evaluator.Close())
		if resultErr != nil {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(manager, result))
		}
	}()
	evalBool := func(expression ast.ASTHandle) (bool, bool, error) {
		value, evalErr := evaluator.EvaluateValue(expression)
		if evalErr != nil {
			return false, false, evalErr
		}
		h, getErr := manager.RefGet(value)
		if getErr != nil {
			return false, false, errors.Join(getErr, manager.DelRef(&value))
		}
		truth := manager.IsTrue(h)
		falsehood := manager.IsFalse(h)
		return truth, falsehood, manager.DelRef(&value)
	}
	truth := func(expression ast.ASTHandle) (bool, error) {
		t, f, e := evalBool(expression)
		return t && !f, e
	}
	falsehood := func(expression ast.ASTHandle) (bool, error) {
		t, f, e := evalBool(expression)
		return f && !t, e
	}
	addLiteral := func(expression ast.ASTHandle) error {
		t, f, evalErr := evalBool(expression)
		if evalErr != nil {
			return evalErr
		}
		if t == f {
			return ast.ErrASTDomain
		}
		literal := expression
		if f {
			literal, evalErr = manager.MkNot(expression)
			if evalErr != nil {
				return evalErr
			}
		}
		// Source canonicalizes a binary distinct literal before arithmetic
		// splitting; larger distinct applications are expanded by the picker.
		literalFamily, e := manager.GetFamilyID(literal)
		if e != nil {
			return e
		}
		literalKind, e := manager.GetDeclKind(literal)
		if e != nil {
			return e
		}
		if literalFamily == manager.BasicFamilyID() && literalKind == ast.ASTOpDistinct {
			n, e := manager.GetNumArgs(literal)
			if e != nil {
				return e
			}
			if n == 2 {
				a, e := manager.GetArg(literal, 0)
				if e != nil {
					return e
				}
				b, e := manager.GetArg(literal, 1)
				if e != nil {
					return e
				}
				eq, e := manager.MkEq(a, b)
				if e != nil {
					return e
				}
				literal, e = manager.MkNot(eq)
				if e != nil {
					return e
				}
			}
		}
		if isNot, inner, getErr := manager.IsNot(literal); getErr != nil {
			return getErr
		} else if isNot {
			innerFamily, familyErr := manager.GetFamilyID(inner)
			if familyErr != nil {
				return familyErr
			}
			kind, kindErr := manager.GetDeclKind(inner)
			if kindErr != nil {
				return kindErr
			}
			if innerFamily == manager.BasicFamilyID() {
				switch kind {
				case ast.ASTOpXor:
					a, x := manager.GetArg(inner, 0)
					b, y := manager.GetArg(inner, 1)
					if x != nil || y != nil {
						return errors.Join(x, y)
					}
					literal, evalErr = manager.MkEq(a, b)
				case ast.ASTOpEq:
					a, x := manager.GetArg(inner, 0)
					b, y := manager.GetArg(inner, 1)
					if x != nil || y != nil {
						return errors.Join(x, y)
					}
					sort, x := manager.GetSort(a)
					if x != nil {
						return x
					}
					family, x := manager.GetFamilyID(sort)
					if x != nil {
						return x
					}
					arithmetic, x := manager.ArithmeticFamilyID()
					if x != nil {
						return x
					}
					// Source add_literal only splits Int/Real disequalities.
					// Every other disequality must still reach the output below.
					if family != arithmetic {
						break
					}
					left, x := manager.MkArithmeticLT(a, b)
					if x != nil {
						return x
					}
					if yes, x := truth(left); x != nil {
						return x
					} else if yes {
						literal = left
					} else {
						literal, evalErr = manager.MkArithmeticLT(b, a)
					}
				}
			}
			if evalErr != nil {
				return evalErr
			}
		}
		if yes, evalErr := truth(literal); evalErr != nil {
			return evalErr
		} else if !yes {
			return ast.ErrASTDomain
		}
		ref, evalErr := manager.NewRef(literal)
		if evalErr == nil {
			result = append(result, ref)
		}
		return evalErr
	}
	var pick func(ast.ASTHandle, map[ast.ASTHandle]bool) error
	pick = func(root ast.ASTHandle, visited map[ast.ASTHandle]bool) error {
		todo := []ast.ASTHandle{root}
		for len(todo) != 0 {
			i := len(todo) - 1
			a := todo[i]
			todo = todo[:i]
			if visited[a] {
				continue
			}
			visited[a] = true
			t, f, evalErr := evalBool(a)
			if evalErr != nil {
				return evalErr
			}
			if !t && !f {
				continue
			}
			family, evalErr := manager.GetFamilyID(a)
			if evalErr != nil {
				return evalErr
			}
			basic := manager.BasicFamilyID()
			if family != basic {
				if evalErr = addLiteral(a); evalErr != nil {
					return evalErr
				}
				continue
			}
			kind, evalErr := manager.GetDeclKind(a)
			if evalErr != nil {
				return evalErr
			}
			count, evalErr := manager.GetNumArgs(a)
			if evalErr != nil {
				return evalErr
			}
			args := make([]ast.ASTHandle, count)
			for j := range args {
				args[j], evalErr = manager.GetArg(a, uint32(j))
				if evalErr != nil {
					return evalErr
				}
			}
			switch kind {
			case ast.ASTOpTrue, ast.ASTOpFalse:
			case ast.ASTOpNot:
				todo = append(todo, args[0])
			case ast.ASTOpDistinct:
				if f {
					found := false
					for j := 0; j < len(args) && !found; j++ {
						vj, e := evaluator.EvaluateValue(args[j])
						if e != nil {
							return e
						}
						valueJ, e := manager.RefGet(vj)
						if e != nil {
							_ = manager.DelRef(&vj)
							return e
						}
						for k := 0; k < j; k++ {
							vk, e := evaluator.EvaluateValue(args[k])
							if e != nil {
								return errors.Join(e, manager.DelRef(&vj))
							}
							valueK, e := manager.RefGet(vk)
							if e != nil {
								return errors.Join(e, manager.DelRef(&vk), manager.DelRef(&vj))
							}
							equal, e := manager.AreEqual(valueJ, valueK)
							if e != nil {
								_ = manager.DelRef(&vk)
								_ = manager.DelRef(&vj)
								return e
							}
							e = manager.DelRef(&vk)
							if e != nil {
								return errors.Join(e, manager.DelRef(&vj))
							}
							if equal {
								eq, e := manager.MkEq(args[j], args[k])
								if e != nil {
									return errors.Join(e, manager.DelRef(&vj))
								}
								todo = append(todo, eq)
								found = true
								break
							}
						}
						err = manager.DelRef(&vj)
						if err != nil {
							return err
						}
					}
					if !found {
						return ast.ErrASTDomain
					}
				} else if len(args) == 2 {
					if evalErr = addLiteral(a); evalErr != nil {
						return evalErr
					}
				} else {
					expanded, e := manager.MkDistinctExpanded(args)
					if e != nil {
						return e
					}
					todo = append(todo, expanded)
				}
			case ast.ASTOpAnd:
				if t {
					todo = append(todo, args...)
				} else {
					for _, child := range args {
						yes, e := falsehood(child)
						if e != nil {
							return e
						}
						if yes {
							todo = append(todo, child)
							break
						}
					}
				}
			case ast.ASTOpOr:
				if f {
					todo = append(todo, args...)
				} else {
					for _, child := range args {
						yes, e := truth(child)
						if e != nil {
							return e
						}
						if yes {
							todo = append(todo, child)
							break
						}
					}
				}
			case ast.ASTOpIff, ast.ASTOpEq:
				if len(args) == 2 {
					equal, e := manager.AreEqual(args[0], args[1])
					if e != nil {
						return e
					}
					distinct, e := manager.AreDistinct(args[0], args[1])
					if e != nil {
						return e
					}
					if !equal && !distinct {
						boolSort, e := manager.MkBoolSort()
						if e != nil {
							return e
						}
						sort0, e := manager.GetSort(args[0])
						if e != nil {
							return e
						}
						sort1, e := manager.GetSort(args[1])
						if e != nil {
							return e
						}
						leftConst, e := implicantUninterpretedConstant(manager, args[0])
						if e != nil {
							return e
						}
						rightConst, e := implicantUninterpretedConstant(manager, args[1])
						if e != nil {
							return e
						}
						if sort0 == boolSort && sort1 == boolSort && (!leftConst || !rightConst) {
							todo = append(todo, args...)
						} else if evalErr = addLiteral(a); evalErr != nil {
							return evalErr
						}
					}
				}
			case ast.ASTOpIte:
				if len(args) == 3 {
					equal, e := manager.AreEqual(args[1], args[2])
					if e != nil {
						return e
					}
					if equal {
						todo = append(todo, args[1])
						continue
					}
					thenTrue, e := truth(args[1])
					if e != nil {
						return e
					}
					elseTrue, e := truth(args[2])
					if e != nil {
						return e
					}
					thenFalse, e := falsehood(args[1])
					if e != nil {
						return e
					}
					elseFalse, e := falsehood(args[2])
					if e != nil {
						return e
					}
					conditionTrue, e := truth(args[0])
					if e != nil {
						return e
					}
					conditionFalse, e := falsehood(args[0])
					if e != nil {
						return e
					}
					switch {
					case thenTrue && elseTrue, thenFalse && elseFalse:
						todo = append(todo, args[1:]...)
					case conditionTrue:
						todo = append(todo, args[0], args[1])
					case conditionFalse:
						todo = append(todo, args[0], args[2])
					}
				}
			case ast.ASTOpXor:
				todo = append(todo, args...)
			case ast.ASTOpImplies:
				if t {
					if yes, e := truth(args[1]); e != nil {
						return e
					} else if yes {
						todo = append(todo, args[1])
					} else if no, e := falsehood(args[0]); e != nil {
						return e
					} else if no {
						todo = append(todo, args[0])
					}
				} else {
					todo = append(todo, args...)
				}
			default:
				if evalErr = addLiteral(a); evalErr != nil {
					return evalErr
				}
			}
		}
		return nil
	}
	// Source flatten_and mutates its owning input vector, including swapping
	// appended children into the current slot and collapsing false conjuncts.
	flattened, err := manager.FlattenAnd(append([]ast.ASTHandle(nil), formulas...))
	if err != nil {
		return nil, err
	}
	flattenedRefs, err := refsFromHandles(manager, flattened)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(manager, flattenedRefs)) }()
	allTrue := true
	if len(flattened) != 0 {
		conjunction, e := manager.MkAndArray(flattened)
		if e != nil {
			return nil, e
		}
		allTrue, err = truth(conjunction)
		if err != nil {
			return nil, err
		}
	}
	visited := make(map[ast.ASTHandle]bool)
	for _, formula := range flattened {
		yes := allTrue
		if !yes {
			yes, err = truth(formula)
			if err != nil {
				return nil, err
			}
		}
		if yes {
			if err = pick(formula, visited); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func implicantUninterpretedConstant(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	family, err := m.GetFamilyID(h)
	if err != nil || family != ast.NullFamilyID {
		return false, err
	}
	count, err := m.GetNumArgs(h)
	return count == 0, err
}
