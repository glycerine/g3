package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter/bit_blaster"
)

// Reconstruct the source macro's circuit using the concrete bit-blaster,
// independently of the theory's dispatch, cache, native literals, and trails.
func bvSourceOperator(t *testing.T, m *ast.ASTManager, family ast.FamilyID, ext bool, op int32, args [][]ast.ASTHandle) []ast.ASTHandle {
	t.Helper()
	config, err := bit_blaster.NewConfig(m, family, bit_blaster.SMTBitBlasterParams{ExtGates: ext})
	astTestMust(t, err)
	engine, err := bit_blaster.NewEngine(config)
	astTestMust(t, err)
	var roots []ast.ASTRef
	t.Cleanup(func() {
		for i := len(roots) - 1; i >= 0; i-- {
			astTestMust(t, m.DelRef(&roots[i]))
		}
	})
	handles := func(refs []ast.ASTRef, err error) []ast.ASTHandle {
		astTestMust(t, err)
		roots = append(roots, refs...)
		bits := make([]ast.ASTHandle, len(refs))
		for i, ref := range refs {
			bits[i], err = m.RefGet(ref)
			astTestMust(t, err)
		}
		return bits
	}
	scalar := func(operation func(ast.ASTRef) error) []ast.ASTHandle {
		root, err := m.NewRef(ast.ASTHandle{})
		astTestMust(t, err)
		roots = append(roots, root)
		astTestMust(t, operation(root))
		h, err := m.RefGet(root)
		astTestMust(t, err)
		return []ast.ASTHandle{h}
	}
	switch op {
	case 4, 6, 30, 31, 33, 34, 35, 36:
		var result []ast.ASTHandle
		for i := len(args) - 1; i >= 0; i-- {
			if i == len(args)-1 {
				result = args[i]
				continue
			}
			switch op {
			case 4:
				result = handles(engine.MkAdder(args[i], result))
			case 6:
				result = handles(engine.MkMultiplier(args[i], result))
			case 30:
				result = handles(engine.MkAnd(args[i], result))
			case 31:
				result = handles(engine.MkOr(args[i], result))
			case 33:
				result = handles(engine.MkXor(args[i], result))
			case 34:
				result = handles(engine.MkNand(args[i], result))
			case 35:
				result = handles(engine.MkNor(args[i], result))
			case 36:
				result = handles(engine.MkXnor(args[i], result))
			}
		}
		return result
	case 5:
		refs, carry, err := engine.MkSubtracter(args[0], args[1])
		astTestMust(t, err)
		roots = append(roots, carry)
		return handles(refs, nil)
	case 17:
		return handles(engine.MkSignedDiv(args[0], args[1]))
	case 18:
		return handles(engine.MkUnsignedDiv(args[0], args[1]))
	case 19:
		return handles(engine.MkSignedRem(args[0], args[1]))
	case 20:
		return handles(engine.MkUnsignedRem(args[0], args[1]))
	case 21:
		return handles(engine.MkSignedMod(args[0], args[1]))
	case 22:
		return scalar(func(ref ast.ASTRef) error { return engine.MkUnsignedLE(args[0], args[1], ref) })
	case 23:
		return scalar(func(ref ast.ASTRef) error { return engine.MkSignedLE(args[0], args[1], ref) })
	case 32:
		return handles(engine.MkNot(args[0]))
	case 37:
		return append(append([]ast.ASTHandle(nil), args[1]...), args[0]...)
	case 38:
		return handles(engine.MkSignExtend(args[0], 2))
	case 39:
		return handles(engine.MkZeroExtend(args[0], 2))
	case 40:
		return append([]ast.ASTHandle(nil), args[0][:1]...)
	case 42:
		return scalar(func(ref ast.ASTRef) error { return engine.MkReductionOr(args[0], ref) })
	case 43:
		return scalar(func(ref ast.ASTRef) error { return engine.MkReductionAnd(args[0], ref) })
	case 44:
		return scalar(func(ref ast.ASTRef) error { return engine.MkComp(args[0], args[1], ref) })
	case 45:
		return handles(engine.MkShiftLeft(args[0], args[1]))
	case 46:
		return handles(engine.MkLogicalShiftRight(args[0], args[1]))
	case 47:
		return handles(engine.MkArithmeticShiftRight(args[0], args[1]))
	case 48:
		return handles(engine.MkRotateLeft(args[0], 3))
	case 49:
		return handles(engine.MkRotateRight(args[0], 3))
	case 50:
		return handles(engine.MkExtendedRotateLeft(args[0], args[1]))
	case 51:
		return handles(engine.MkExtendedRotateRight(args[0], args[1]))
	case 52:
		return scalar(func(ref ast.ASTRef) error { return engine.MkUnsignedMulNoOverflow(args[0], args[1], ref) })
	case 53:
		return scalar(func(ref ast.ASTRef) error { return engine.MkSignedMulNoOverflow(args[0], args[1], ref) })
	case 54:
		return scalar(func(ref ast.ASTRef) error { return engine.MkSignedMulNoUnderflow(args[0], args[1], ref) })
	default:
		t.Fatal("missing source operator oracle", op)
		return nil
	}
}

// Intern the circuit DAG rather than expanding shared subgraphs into a tree.
// Both contexts start from the same AST construction frontier. Gate operand
// order is preserved, including AST-ID-dependent expanded carry gates.
func bvSourceCircuits(t *testing.T, m *ast.ASTManager, roots []ast.ASTHandle, intern map[string]int) []int {
	t.Helper()
	memo := make(map[ast.ASTHandle]int)
	var visit func(ast.ASTHandle) int
	visit = func(h ast.ASTHandle) int {
		if id, ok := memo[h]; ok {
			return id
		}
		count, err := m.GetNumArgs(h)
		astTestMust(t, err)
		var key string
		if count == 0 {
			id, err := m.GetASTID(h)
			astTestMust(t, err)
			key = fmt.Sprint("leaf", id)
		} else {
			op, err := m.GetDeclKind(h)
			astTestMust(t, err)
			family, err := m.GetFamilyID(h)
			astTestMust(t, err)
			args := make([]int, count)
			for i := range args {
				arg, err := m.GetArg(h, uint32(i))
				astTestMust(t, err)
				args[i] = visit(arg)
			}
			key = fmt.Sprint(family, ":", op, ":", args)
		}
		id, ok := intern[key]
		if !ok {
			id = len(intern) + 1
			intern[key] = id
		}
		memo[h] = id
		return id
	}
	result := make([]int, len(roots))
	for i, root := range roots {
		result[i] = visit(root)
	}
	return result
}

func TestSMTBVNativeOperatorCircuitsMatchSourceEngine(t *testing.T) {
	for _, ext := range []bool{false, true} {
		for _, width := range []int{1, 4} {
			for _, op := range []int32{4, 5, 6, 17, 18, 19, 20, 21, 22, 23, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54} {
				t.Run(fmt.Sprint(ext, "/", width, "/", op), func(t *testing.T) {
					prepare := func() (*ast.ASTManager, *SMTInternalizerState, *SMTBitVectorTheory, ast.ASTHandle, [][]ast.ASTHandle) {
						m, s, bv := bvOccurrenceFixture(t)
						bv.bitBlasterParams.ExtGates = ext
						var words []ast.ASTHandle
						var inputs [][]ast.ASTHandle
						count := 2
						switch op {
						case 4, 6, 30, 31, 33:
							count = 3
						case 32, 38, 39, 40, 42, 43, 48, 49:
							count = 1
						}
						for operand := 0; operand < count; operand++ {
							bits := make([]ast.ASTHandle, width)
							for bit := range bits {
								bits[bit] = smtRelevancyBool(t, m, fmt.Sprint("native-op-", operand, "-", bit))
								if (operand+bit)%2 != 0 {
									var err error
									bits[bit], err = m.MkNot(bits[bit])
									astTestMust(t, err)
								}
							}
							node, _ := bvNativeWord(t, m, s, bv, bits...)
							words = append(words, node.owner)
							inputs = append(inputs, bits)
						}
						integer := func(value int32) ast.ASTParameter {
							p, err := m.NewParameterInt(value)
							astTestMust(t, err)
							t.Cleanup(func() { astTestMust(t, m.DelParameter(&p)) })
							return p
						}
						var parameters []ast.ASTParameter
						switch op {
						case 38, 39:
							parameters = []ast.ASTParameter{integer(2)}
						case 40:
							parameters = []ast.ASTParameter{integer(0), integer(0)}
						case 48, 49:
							parameters = []ast.ASTParameter{integer(3)}
						}
						term, err := m.MkFamilyApp(bv.FamilyID(), op, parameters, words)
						astTestMust(t, err)
						return m, s, bv, term, inputs
					}
					expectedManager, _, expectedBV, _, inputs := prepare()
					expected := bvSourceOperator(t, expectedManager, expectedBV.FamilyID(), ext, op, inputs)
					intern := make(map[string]int)
					want := bvSourceCircuits(t, expectedManager, expected, intern)
					m, s, bv, term, _ := prepare()
					astTestMust(t, s.Internalize(term, false))
					var actual []SMTLiteral
					if op == 22 || op == 23 || op >= 52 {
						variable, ok := s.BoolVar(term)
						if !ok {
							t.Fatal("missing native comparison atom")
						}
						definition, ok := bv.comparisonAtoms[variable]
						if !ok {
							t.Fatal("missing native comparison definition")
						}
						actual = []SMTLiteral{definition}
					} else {
						actual = bv.bitLiterals[s.context.enodes.get(term).TheoryVar(bv.FamilyID())]
					}
					if len(actual) != len(expected) {
						t.Fatal("source output width", len(actual), len(expected))
					}
					expressions := make([]ast.ASTHandle, len(actual))
					for i, literal := range actual {
						var err error
						expressions[i], err = s.literalExpression(literal)
						astTestMust(t, err)
					}
					got := bvSourceCircuits(t, m, expressions, intern)
					if fmt.Sprint(got) != fmt.Sprint(want) {
						t.Fatal("native source circuit mismatch", op, got, want)
					}
					if len(bv.temporary) != 0 {
						t.Fatal("native output references escaped publication", len(bv.temporary))
					}
				})
			}
		}
	}
}

func TestSMTBVNativeOperatorRefusalScopeAndRetry(t *testing.T) {
	for _, op := range []int32{6, 18, 53} {
		failures := 0
		for step := int64(0); step < 32; step++ {
			t.Run(fmt.Sprint(op, "/", step), func(t *testing.T) {
				m, s, bv := bvOccurrenceFixture(t)
				x, _ := bvFixedSymbolic(t, m, s, bv, "operator-refusal-x", 8)
				y, _ := bvFixedSymbolic(t, m, s, bv, "operator-refusal-y", 8)
				term, err := m.MkFamilyApp(bv.FamilyID(), op, nil, []ast.ASTHandle{x.owner, y.owner})
				astTestMust(t, err)
				root, err := m.NewRef(term)
				astTestMust(t, err)
				defer func() { astTestMust(t, m.DelRef(&root)) }()
				base := len(bv.variables)
				astTestMust(t, s.PushScope())
				m.Budget().FailAfter(step)
				err = s.Internalize(term, false)
				m.Budget().FailAfter(-1)
				if err != nil {
					failures++
				}
				astTestMust(t, s.PopScopes(1))
				if len(bv.variables) != base || len(bv.temporary) != 0 || s.context.enodes.get(term) != nil {
					t.Fatal("operator refusal escaped its scope")
				}
				astTestMust(t, s.PushScope())
				astTestMust(t, s.Internalize(term, false))
				if len(bv.temporary) != 0 {
					t.Fatal("operator retry retained temporary output references")
				}
				if op == 53 {
					variable, ok := s.BoolVar(term)
					if !ok {
						t.Fatal("missing retried comparison")
					}
					if _, ok = bv.comparisonAtoms[variable]; !ok {
						t.Fatal("missing retried comparison definition")
					}
				} else if node := s.context.enodes.get(term); node == nil || len(bv.bitLiterals[node.TheoryVar(bv.FamilyID())]) != 8 {
					t.Fatal("missing retried native result")
				}
				astTestMust(t, s.PopScopes(1))
			})
		}
		if failures == 0 {
			t.Fatal("native operator never reached a cold injected refusal", op)
		}
	}
}

func TestSMTBVNativeOperandLookupCreatesSuppressedVariable(t *testing.T) {
	m, s, bv := bvOccurrenceFixture(t)
	bv.config.BlastMaxSize = 2
	sort, err := m.BitVectorSort(4)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "native-suppressed-operand"), sort)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(x, false))
	node := s.context.enodes.get(x)
	if node == nil || node.TheoryVar(bv.FamilyID()) != SMTNullTheoryVar || !bv.unsupported {
		t.Fatal("fixture did not suppress sort attachment")
	}
	astTestMust(t, s.PushScope())
	call := smtBVExpressionCall{theory: bv}
	bits, err := call.bits(x)
	astTestMust(t, err)
	astTestMust(t, call.close())
	v := node.TheoryVar(bv.FamilyID())
	if v == SMTNullTheoryVar || len(bits) != 4 || len(bv.bitLiterals[v]) != 4 || bv.watchedPosition[v] != 0 {
		t.Fatal("source get_var did not create the native bit vector")
	}
	for i, literal := range bv.bitLiterals[v] {
		atom, err := m.MkBitVectorBit2Bool(uint32(i), x)
		astTestMust(t, err)
		variable, ok := s.BoolVar(atom)
		if !ok || literal != NewSMTLiteral(variable, false) || len(bv.bitAtoms[variable]) != 0 {
			t.Fatal("source mk_bits atom/position", i, literal)
		}
		data, err := s.BooleanData(variable)
		astTestMust(t, err)
		if data.Theory() != SMTNullTheoryID {
			t.Fatal("approximated bit unexpectedly attached to BV theory", i, data.Theory())
		}
	}
	if len(bv.fixedVariables) != 0 {
		t.Fatal("source mk_bits unexpectedly ran fixed-value notification")
	}
	astTestMust(t, s.PopScopes(1))
	if node.TheoryVar(bv.FamilyID()) != SMTNullTheoryVar || len(bv.variables) != 0 || len(bv.bitAtoms) != 0 {
		t.Fatal("native operand variable did not retire with its scope")
	}
	status, err := bv.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckGiveUp {
		t.Fatal("native get_var lost the earlier approximation flag")
	}
}

func TestSMTBVAtomApproximationPrecedesOperatorDispatch(t *testing.T) {
	m, _, bv := bvOccurrenceFixture(t)
	bv.config.BlastMaxSize = 2
	sort, err := m.BitVectorSort(4)
	astTestMust(t, err)
	left, err := m.MkConst(astTestSymbol(t, m, "approximate-atom-left"), sort)
	astTestMust(t, err)
	right, err := m.MkConst(astTestSymbol(t, m, "approximate-atom-right"), sort)
	astTestMust(t, err)
	comparison, err := m.MkBitVectorULE(left, right)
	astTestMust(t, err)
	handled, err := bv.InternalizeAtom(comparison, true)
	astTestMust(t, err)
	if handled || !bv.unsupported || len(bv.comparisonAtoms) != 0 {
		t.Fatal("source atom approximation did not stop before operator dispatch", handled, bv.unsupported, len(bv.comparisonAtoms))
	}
	if bv.state.context.enodes.get(left) != nil || bv.state.context.enodes.get(right) != nil {
		t.Fatal("source atom approximation processed operands before refusal")
	}
}
