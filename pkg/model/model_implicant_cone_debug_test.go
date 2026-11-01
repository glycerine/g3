package model

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Source: model_implicant::minimize_model's DEBUG_CODE block, cpp:109-114.
func TestModelImplicantConeRechecksModelInSourceDebugBuild(t *testing.T) {
	m, md, implicant, output, tags := modelImplicantDiagnosticFixture(t)
	var pins []ast.ASTRef
	defer func() {
		for n := range pins {
			arrayTableMust(t, m.DelRef(&pins[n]))
		}
	}()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		arrayTableMust(t, err)
		r, err := m.NewRef(h)
		arrayTableMust(t, err)
		pins = append(pins, r)
		return h
	}
	boolean := own(m.MkBoolSort())
	_ = own(m.MkTrue())
	no := own(m.MkFalse())
	sort := own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean))
	value := own(m.MkConstArray(sort, no))
	symbol, err := m.Symbols.Intern("cone-debug-unselected")
	arrayTableMust(t, err)
	variable := own(m.MkConst(symbol, sort))
	declaration, err := m.GetDecl(variable)
	arrayTableMust(t, err)
	arrayTableMust(t, md.Core().RegisterConstant(declaration, value))
	functionSymbol, err := m.Symbols.Intern("cone-debug-predicate")
	arrayTableMust(t, err)
	function := own(m.MkFuncDecl(functionSymbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
	bound := own(m.MkVar(0, boolean))
	body := own(m.MkNot(bound))
	graph, err := NewFunctionInterpretation(m, 1)
	arrayTableMust(t, err)
	arrayTableMust(t, graph.SetElse(body))
	arrayTableMust(t, md.Core().RegisterFunction(function, graph))
	formula := own(m.MkApp(function, []ast.ASTHandle{no}))
	tags.Enable("model_evaluator")
	tags.Enable("pdr")
	before := m.Stats()
	for attempt := 0; attempt < 2; attempt++ {
		output.Reset()
		result, err := implicant.MinimizeModel([]ast.ASTHandle{formula}, md)
		arrayTableMust(t, err)
		if len(result) != 0 {
			t.Fatal("unrelated array constant entered the cone")
		}
		arrayTableMust(t, CloseImplicantResults(m, &result))
		if tags.ShouldTrace("pdr") {
			want := 1
			if ast.ASTSourceDebug {
				want = 2
			}
			if got := strings.Count(output.String(), "[pdr] assign_value src/model/model_implicant.cpp:66"); got != want {
				t.Fatal("wrong number of source setup_model passes", got, want, output.String())
			}
		}
		if tags.ShouldTrace("model_evaluator") {
			want := 1
			if ast.ASTSourceDebug {
				want = 2
			}
			if got := strings.Count(output.String(), "[model_evaluator] operator() src/model/model_evaluator.cpp:566"); got != want {
				t.Fatal("wrong number of model truth evaluations", got, want, output.String())
			}
		}
		if implicant.model != nil || implicant.evaluator != nil || len(implicant.refs) != 0 || len(implicant.states) != 0 {
			t.Fatal("debug verification left active model state")
		}
		after := m.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
			t.Fatal("debug model recheck leaked ownership", after, before)
		}
	}
}
