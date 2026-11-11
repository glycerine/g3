package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelFinderInstantiationSetOwnershipInverseAndRefusals(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	set, err := newSMTModelFinderInstantiationSet(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, set.Close()) })
	inserted, err := set.Insert(zero, 4)
	astTestMust(t, err)
	if !inserted {
		t.Fatal("first instantiation term was not inserted")
	}
	inserted, err = set.Insert(zero, 1)
	astTestMust(t, err)
	if inserted || set.testElements()[zero] != 4 {
		t.Fatal("duplicate changed source generation", inserted, set.testElements()[zero])
	}
	modelValue, err := m.MkModelValue(0, integer)
	astTestMust(t, err)
	inserted, err = set.Insert(modelValue, 0)
	astTestMust(t, err)
	if inserted {
		t.Fatal("private model value entered instantiation set")
	}
	inserted, err = set.Insert(one, 2)
	astTestMust(t, err)
	if !inserted {
		t.Fatal("second instantiation term was not inserted")
	}
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	astTestMust(t, set.MakeInverse(smtTestProtoEval(t, m, proto)))
	term, generation, found, err := set.Inverse(one)
	astTestMust(t, err)
	if !found || term != one || generation != 2 {
		t.Fatal("inverse lookup", term, generation, found)
	}
	if err = set.Remove(one); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("term removed after inverse construction", err)
	}
	astTestMust(t, set.MakeInverse(smtTestProtoEval(t, m, proto)))
}

func TestSMTModelFinderInstantiationSetRejectsQuantifiedPrivateValues(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "instantiation-private-sort"), nil)
	astTestMust(t, err)
	private, err := m.MkModelValue(0, sort)
	astTestMust(t, err)
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate, err := m.MkFuncDecl(astTestSymbol(t, m, "instantiation-private-p"), []ast.ASTHandle{sort, sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	application, err := m.MkApp(predicate, []ast.ASTHandle{x, private})
	astTestMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{application})
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	set, err := newSMTModelFinderInstantiationSet(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, set.Close()) })
	for _, location := range []string{"body", "pattern", "no-pattern"} {
		spec := ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "instantiation-private-x")}, Body: truth}
		switch location {
		case "body":
			spec.Body = application
		case "pattern":
			spec.Patterns = []ast.ASTHandle{pattern}
		case "no-pattern":
			spec.NoPatterns = []ast.ASTHandle{application}
		}
		quantifier, err := m.MkQuantifier(spec)
		astTestMust(t, err)
		inserted, err := set.Insert(quantifier, 0)
		astTestMust(t, err)
		if inserted {
			t.Fatal("private model value escaped through quantified " + location)
		}
	}
	if set.size() != 0 {
		t.Fatal("rejected private terms retained ownership")
	}
}

// smtTestProtoEval is an evaluator over proto (model completion) whose
// results live until the test ends.
func smtTestProtoEval(t *testing.T, m *ast.ASTManager, proto *protomodel.ProtoModel) func(ast.ASTHandle) (ast.ASTHandle, error) {
	return func(term ast.ASTHandle) (ast.ASTHandle, error) {
		ref, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return ast.ASTHandle{}, err
		}
		t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
		ok, err := proto.Eval(term, ref, true)
		if err != nil || !ok {
			return ast.ASTHandle{}, err
		}
		return m.RefGet(ref)
	}
}
