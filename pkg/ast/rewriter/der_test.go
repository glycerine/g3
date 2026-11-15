package rewriter

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestDERVariableDisequalityEncodings(t *testing.T) {
	m := astTestManager(t)
	d, err := NewDER(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, d.Close()) })
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	intFamily, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	intSort, err := m.MkFamilySort(intFamily, 1, nil)
	astTestMust(t, err)
	astTestPin(t, m, intSort)
	iv, err := m.MkVar(0, intSort)
	astTestMust(t, err)
	bv, err := m.MkVar(0, boolSort)
	astTestMust(t, err)
	outside, err := m.MkVar(2, boolSort)
	astTestMust(t, err)
	p, err := m.MkConst(astTestSymbol(t, m, "der-p"), boolSort)
	astTestMust(t, err)
	astTestPin(t, m, p)
	one := bit2IntArithmeticNumeral(t, m, "1")
	eq, err := m.MkEq(iv, one)
	astTestMust(t, err)
	notEq, err := m.MkNot(eq)
	astTestMust(t, err)
	iff, err := m.MkIff(bv, p)
	astTestMust(t, err)
	notBV, err := m.MkNot(bv)
	astTestMust(t, err)
	negatedIff, err := m.MkIff(notBV, p)
	astTestMust(t, err)
	call := derCall{owner: d}
	defer func() { astTestMust(t, call.close()) }()

	for _, row := range []struct {
		name       string
		term, want ast.ASTHandle
		index      uint32
	}{
		{"not equality", notEq, one, 0},
		{"negative Boolean variable", notBV, ast.ASTHandle{}, 0},
		{"positive Boolean variable", bv, ast.ASTHandle{}, 0},
		{"negated iff variable", negatedIff, p, 0},
	} {
		variable, index, definition, matched, err := d.isVarDiseq(&call, row.term, 1)
		astTestMust(t, err)
		if !matched || index != row.index || variable == (ast.ASTHandle{}) {
			t.Fatal(row.name, "recognition", matched, index, variable)
		}
		switch row.name {
		case "positive Boolean variable":
			want, err := m.MkFalse()
			astTestMust(t, err)
			if definition != want {
				t.Fatal(row.name, definition, want)
			}
		case "negative Boolean variable":
			want, err := m.MkTrue()
			astTestMust(t, err)
			if definition != want {
				t.Fatal(row.name, definition, want)
			}
		default:
			if definition != row.want {
				t.Fatal(row.name, definition, row.want)
			}
		}
	}
	_, _, definition, matched, err := d.isVarDiseq(&call, iff, 1)
	astTestMust(t, err)
	if !matched {
		t.Fatal("iff variable was not recognized")
	}
	isNot, err := d.isBasic(definition, ast.ASTOpNot)
	astTestMust(t, err)
	if !isNot {
		t.Fatal("iff definition was not complemented")
	}
	_, _, _, matched, err = d.isVarDiseq(&call, outside, 1)
	astTestMust(t, err)
	if matched {
		t.Fatal("variable outside declaration range accepted")
	}
}

func TestDERSortVariablesOrderCyclesAndExclusions(t *testing.T) {
	f := newFactorFixture(t)
	d, err := NewDER(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, d.Close()) })
	variables := make([]ast.ASTHandle, 4)
	for i := range variables {
		variables[i], err = f.m.MkVar(uint32(i), f.sort)
		astTestMust(t, err)
	}
	one := f.numeral(1)
	definitions := []ast.ASTHandle{
		f.app(6, variables[1], one),
		f.app(6, variables[2], one),
		one,
		f.app(6, variables[3], one),
	}
	order, err := d.sortVariables(variables, definitions)
	astTestMust(t, err)
	if !reflect.DeepEqual(order, []uint32{2, 1, 0}) {
		t.Fatal("acyclic elimination order", order)
	}
	if definitions[3] != (ast.ASTHandle{}) {
		t.Fatal("self-loop definition survived")
	}

	cycleDefinitions := []ast.ASTHandle{
		f.app(6, variables[1], one),
		f.app(6, variables[0], one),
		one,
		ast.ASTHandle{},
	}
	order, err = d.sortVariables(variables, cycleDefinitions)
	astTestMust(t, err)
	if cycleDefinitions[0] != (ast.ASTHandle{}) {
		t.Fatal("cycle entry was not removed")
	}
	if !reflect.DeepEqual(order, []uint32{1, 2}) {
		t.Fatal("cycle-breaking order", order)
	}

	boolSort, err := f.m.MkBoolSort()
	astTestMust(t, err)
	bv, err := f.m.MkVar(0, boolSort)
	astTestMust(t, err)
	quantifier := elimBoundsQuantifier(t, f, true, []ast.ASTHandle{boolSort}, bv)
	quantifiedDefinitions := []ast.ASTHandle{quantifier, ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}}
	order, err = d.sortVariables(variables, quantifiedDefinitions)
	astTestMust(t, err)
	if len(order) != 0 || quantifiedDefinitions[0] != (ast.ASTHandle{}) {
		t.Fatal("quantified definition survived", order, quantifiedDefinitions[0])
	}
}

func TestDERUnitAndMultiEqualityReduction(t *testing.T) {
	m := astTestManager(t)
	d, err := NewDER(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, d.Close()) })
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	bv, err := m.MkVar(0, boolSort)
	astTestMust(t, err)
	unit := elimBoundsQuantifier(t, factorFixture{t: t, m: m}, true, []ast.ASTHandle{boolSort}, bv)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	proof, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() {
		astTestMust(t, m.DelRef(&proof))
		astTestMust(t, m.DelRef(&out))
	}()
	astTestMust(t, d.ReduceQuantifier(unit, out, proof))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	falseValue, err := m.MkFalse()
	astTestMust(t, err)
	if got != falseValue {
		t.Fatal("unit DER result", got, falseValue)
	}

	f := newFactorFixture(t)
	x, err := f.m.MkVar(0, f.sort)
	astTestMust(t, err)
	y, err := f.m.MkVar(1, f.sort)
	astTestMust(t, err)
	eq, err := f.m.MkEq(x, y)
	astTestMust(t, err)
	diseq, err := f.m.MkNot(eq)
	astTestMust(t, err)
	boolSort, err = f.m.MkBoolSort()
	astTestMust(t, err)
	predicate, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "der-int-p"), []ast.ASTHandle{f.sort}, boolSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, f.m, predicate)
	px, err := f.m.MkApp(predicate, []ast.ASTHandle{x})
	astTestMust(t, err)
	body, err := f.m.MkOr(diseq, px)
	astTestMust(t, err)
	q, err := f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.sort, f.sort}, Names: []usymbol.Symbol{astTestSymbol(t, f.m, "x"), astTestSymbol(t, f.m, "y")}, Body: body, NoPatterns: []ast.ASTHandle{px}})
	astTestMust(t, err)
	astTestPin(t, f.m, q)
	d2, err := NewDER(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, d2.Close()) })
	out2, err := f.m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	proof2, err := f.m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() {
		astTestMust(t, f.m.DelRef(&proof2))
		astTestMust(t, f.m.DelRef(&out2))
	}()
	astTestMust(t, d2.ReduceQuantifier(q, out2, proof2))
	got, err = f.m.RefGet(out2)
	astTestMust(t, err)
	if count, err := f.m.GetNumDeclarations(got); err != nil || count != 1 {
		t.Fatal("multi-equality declaration count", count, err)
	}
	newBody, err := f.m.GetQuantifierBody(got)
	astTestMust(t, err)
	arg, err := f.m.GetArg(newBody, 0)
	astTestMust(t, err)
	if index, err := f.m.GetVarIndex(arg); err != nil || index != 0 {
		t.Fatal("multi-equality body substitution", index, err)
	}
	noPattern, err := f.m.GetNoPattern(got, 0)
	astTestMust(t, err)
	arg, err = f.m.GetArg(noPattern, 0)
	astTestMust(t, err)
	if index, err := f.m.GetVarIndex(arg); err != nil || index != 0 {
		t.Fatal("multi-equality annotation substitution", index, err)
	}
}

func TestDERRewriterNestedCycleAndProofEntry(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	x, err := m.MkVar(0, boolSort)
	astTestMust(t, err)
	y, err := m.MkVar(1, boolSort)
	astTestMust(t, err)
	xy, err := m.MkIff(x, y)
	astTestMust(t, err)
	yx, err := m.MkIff(y, x)
	astTestMust(t, err)
	p, err := m.MkConst(astTestSymbol(t, m, "der-cycle-p"), boolSort)
	astTestMust(t, err)
	astTestPin(t, m, p)
	body, err := m.MkOrArray([]ast.ASTHandle{xy, yx, p})
	astTestMust(t, err)
	q := elimBoundsQuantifier(t, factorFixture{t: t, m: m}, true, []ast.ASTHandle{boolSort, boolSort}, body)
	wrapped, err := m.MkNot(q)
	astTestMust(t, err)
	astTestPin(t, m, wrapped)
	rw, err := NewDERRewriter(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	proof, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() {
		astTestMust(t, m.DelRef(&proof))
		astTestMust(t, m.DelRef(&out))
	}()
	astTestMust(t, rw.Rewrite(wrapped, out, proof))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	child, err := m.GetArg(got, 0)
	astTestMust(t, err)
	if child == q {
		t.Fatal("nested cyclic DER made no progress")
	}
	if declarations, err := m.GetNumDeclarations(child); err != nil || declarations != 1 {
		t.Fatal("cycle-breaking declaration count", declarations, err)
	}
	pHandle, err := m.RefGet(proof)
	astTestMust(t, err)
	if pHandle == (ast.ASTHandle{}) {
		t.Fatal("DER rewrite proof missing")
	}
	astTestMust(t, rw.Reset())
}

// AUDIT.md ast_rewriter_03-F1: der::reduce1 reads the direct arguments of
// the top-level or and apply_substitution keeps the surviving ones in
// their original order (no flatten_or reordering).
func TestDERKeepsDirectOrArgumentOrder(t *testing.T) {
	f := newFactorFixture(t)
	d, err := NewDER(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, d.Close()) })
	boolean, err := f.m.MkBoolSort()
	astTestMust(t, err)
	pDecl, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "der-order-p"), []ast.ASTHandle{f.sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, f.m, pDecl)
	qDecl, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "der-order-q"), []ast.ASTHandle{f.sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, f.m, qDecl)
	y, err := f.m.MkVar(0, f.sort)
	astTestMust(t, err)
	x, err := f.m.MkVar(1, f.sort)
	astTestMust(t, err)
	py, err := f.m.MkApp(pDecl, []ast.ASTHandle{y})
	astTestMust(t, err)
	eq, err := f.m.MkEq(x, f.numeral(5))
	astTestMust(t, err)
	diseq, err := f.m.MkNot(eq)
	astTestMust(t, err)
	qx, err := f.m.MkApp(qDecl, []ast.ASTHandle{x})
	astTestMust(t, err)
	body, err := f.m.MkOrArgs([]ast.ASTHandle{py, diseq, qx})
	astTestMust(t, err)
	got := derReduce(t, d, f.m, derQuantifier(t, f.m, true, []ast.ASTHandle{f.sort, f.sort}, body))
	if count, e := f.m.GetNumDeclarations(got); e != nil || count != 1 {
		t.Fatal("remaining declarations", count, e)
	}
	reduced, err := f.m.GetQuantifierBody(got)
	astTestMust(t, err)
	for i, want := range []ast.ASTHandle{pDecl, qDecl} {
		arg, e := f.m.GetArg(reduced, uint32(i))
		astTestMust(t, e)
		decl, e := f.m.GetDecl(arg)
		astTestMust(t, e)
		if decl != want {
			t.Fatal("or argument order", i, decl, want)
		}
	}
}

// AUDIT.md ast_rewriter_03-F1: the 4.7.1 unit rule has no has_quantifiers
// test: (forall ((x Bool)) (not (= x (forall ...)))) is false.
func TestDERUnitIgnoresQuantifiedDefinition(t *testing.T) {
	f := newFactorFixture(t)
	d, err := NewDER(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, d.Close()) })
	boolean, err := f.m.MkBoolSort()
	astTestMust(t, err)
	pDecl, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "der-unit-p"), []ast.ASTHandle{f.sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, f.m, pDecl)
	y, err := f.m.MkVar(0, f.sort)
	astTestMust(t, err)
	py, err := f.m.MkApp(pDecl, []ast.ASTHandle{y})
	astTestMust(t, err)
	inner := derQuantifier(t, f.m, true, []ast.ASTHandle{f.sort}, py)
	x, err := f.m.MkVar(0, boolean)
	astTestMust(t, err)
	eq, err := f.m.MkEq(x, inner)
	astTestMust(t, err)
	diseq, err := f.m.MkNot(eq)
	astTestMust(t, err)
	falseValue, err := f.m.MkFalse()
	astTestMust(t, err)
	if got := derReduce(t, d, f.m, derQuantifier(t, f.m, true, []ast.ASTHandle{boolean}, diseq)); got != falseValue {
		t.Fatal("unit rule with quantified definition", got)
	}
}
