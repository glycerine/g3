package model

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func modelImplicantDiagnosticFixture(t *testing.T) (*ast.ASTManager, *Model, *ModelImplicant, *bytes.Buffer, *diagnostic.TraceTags) {
	t.Helper()
	m := arrayTableManager(t)
	arrayTableMust(t, m.RegisterDeclarationPlugins())
	context, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	arrayTableMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	arrayTableMust(t, err)
	globals, err := uparams.NewGlobalParamsService(m.Symbols, context, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	arrayTableMust(t, err)
	arrayTableMust(t, globals.InitializeSource())
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: m.Budget()})
	arrayTableMust(t, err)
	cooperation, err := cooperate.NewCooperation(m.Budget())
	arrayTableMust(t, err)
	t.Cleanup(func() {
		arrayTableMust(t, cooperation.Close())
		arrayTableMust(t, runtime.Close())
		arrayTableMust(t, globals.Close())
		arrayTableMust(t, catalog.Close())
		arrayTableMust(t, context.Close())
	})
	md, err := NewModel(m)
	arrayTableMust(t, err)
	t.Cleanup(func() { arrayTableMust(t, md.Close()) })
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	implicant, err := NewModelImplicant(m, ModelEvaluatorConfig{Context: context, Globals: globals, Memory: runtime, Cooperation: cooperation, Trace: tags})
	arrayTableMust(t, err)
	t.Cleanup(func() { arrayTableMust(t, implicant.Close()) })
	return m, md, implicant, &output, tags
}

func TestModelImplicantArrayExtractorSourceTrace(t *testing.T) {
	for _, name := range []string{"constant", "function", "no-else", "non-ground-entry", "non-ground-else", "untranslated"} {
		t.Run(name, func(t *testing.T) {
			m, md, implicant, output, tags := modelImplicantDiagnosticFixture(t)
			var pins []ast.ASTRef
			defer func() {
				for j := range pins {
					arrayTableMust(t, m.DelRef(&pins[j]))
				}
			}()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				arrayTableMust(t, err)
				ref, err := m.NewRef(h)
				arrayTableMust(t, err)
				pins = append(pins, ref)
				return h
			}
			boolean := own(m.MkBoolSort())
			no, yes := own(m.MkFalse()), own(m.MkTrue())
			symbol, err := m.Symbols.Intern("extraction-f")
			arrayTableMust(t, err)
			function := own(m.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
			graph, err := NewFunctionInterpretation(m, 1)
			arrayTableMust(t, err)
			arrayTableMust(t, md.Core().RegisterFunction(function, graph))
			variable := own(m.MkVar(0, boolean))
			if name != "no-else" {
				value := no
				if name == "non-ground-else" {
					value = variable
				}
				arrayTableMust(t, graph.SetElse(value))
			}
			if name == "non-ground-entry" {
				err = graph.InsertEntry([]ast.ASTHandle{yes}, variable)
				if ast.ASTSourceDebug {
					if err != ast.ErrASTDomain {
						t.Fatal("debug accepted non-ground entry", err)
					}
					return
				}
				arrayTableMust(t, err)
			} else {
				arrayTableMust(t, graph.InsertEntry([]ast.ASTHandle{yes}, yes))
			}
			array := own(m.MkAsArray(function))
			sort, err := m.GetSort(array)
			arrayTableMust(t, err)
			root := array
			entry := "(_ as-array extraction-f)\n"
			line, body, success := 526, "else case: false\n", true
			switch name {
			case "constant":
				root = own(m.MkConstArray(sort, no))
				entry = "((as const (Array Bool Bool)) false)\n"
				line = 0
			case "no-else":
				line, body, success = 515, "no else case (_ as-array extraction-f)\n", false
			case "non-ground-entry":
				line, body, success = 506, "could not extract array interpretation: (_ as-array extraction-f)\n(:var 0)\n", false
			case "non-ground-else":
				line, body, success = 519, "non-ground else case (_ as-array extraction-f)\n(:var 0)\n", false
			case "untranslated":
				symbol, err := m.Symbols.Intern("extraction-unknown")
				arrayTableMust(t, err)
				root = own(m.MkConst(symbol, sort))
				entry = "extraction-unknown\n"
				line, body, success = 529, "no translation: extraction-unknown\n", false
			}
			arrayTableMust(t, implicant.setupModel(md))
			for _, enabled := range []bool{false, true} {
				output.Reset()
				if enabled {
					tags.Enable("pdr")
				}
				before := m.Stats().LiveReferences
				frame, ok, err := implicant.extractArray(root)
				arrayTableMust(t, err)
				arrayTableMust(t, frame.close(m))
				if ok != success {
					t.Fatal("wrong extraction status", ok, success)
				}
				if got := m.Stats().LiveReferences; got != before {
					t.Fatal("trace extraction leaked reference slots", got, before)
				}
				expected := ""
				if enabled && tags.ShouldTrace("pdr") {
					record := func(n int, text string) string {
						return fmt.Sprintf("-------- [pdr] extract_array_func_interp src/model/model_implicant.cpp:%d ---------\n%s------------------------------------------------\n", n, text)
					}
					expected = record(480, entry)
					if line != 0 {
						expected += record(line, body)
					}
				}
				if output.String() != expected {
					t.Fatalf("wrong extractor trace\ngot %q\nwant %q", output.String(), expected)
				}
				if strings.Contains(output.String(), "model_evaluator") {
					t.Fatal("wrong trace tag")
				}
			}
		})
	}
}
