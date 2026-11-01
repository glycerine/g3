package model

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func modelImplicantVerificationFixture(t *testing.T, m *ast.ASTManager, choice string) (*os.File, *diagnostic.NumericCharInput) {
	t.Helper()
	region, err := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: m.Budget()}})
	arrayTableMust(t, err)
	debug, err := diagnostic.NewDebugContext(region, m.Budget())
	arrayTableMust(t, err)
	inputFile, err := os.CreateTemp(t.TempDir(), "verify-input-*")
	arrayTableMust(t, err)
	_, err = inputFile.WriteString(choice)
	arrayTableMust(t, err)
	_, err = inputFile.Seek(0, 0)
	arrayTableMust(t, err)
	output, err := os.CreateTemp(t.TempDir(), "verify-output-*")
	arrayTableMust(t, err)
	streams, err := diagnostic.NewMemoryStandardStreams(diagnostic.MemoryStandardStreamsConfig{Budget: m.Budget(), Stdout: output, Stderr: output})
	arrayTableMust(t, err)
	input, err := diagnostic.NewDebuggerCharInput(streams, inputFile)
	arrayTableMust(t, err)
	action, err := diagnostic.NewDebugActionContext(diagnostic.DebugActionConfig{Budget: m.Budget(), Streams: streams, Input: input})
	arrayTableMust(t, err)
	arrayTableMust(t, debug.BindAssertionStreams(streams))
	arrayTableMust(t, debug.BindAssertionActions(action))
	previous, err := diagnostic.BindProcessDebugContext(debug)
	arrayTableMust(t, err)
	t.Cleanup(func() {
		_, err := diagnostic.BindProcessDebugContext(previous)
		arrayTableMust(t, err)
		arrayTableMust(t, action.Close())
		arrayTableMust(t, input.Close())
		arrayTableMust(t, streams.Close())
		arrayTableMust(t, inputFile.Close())
		arrayTableMust(t, output.Close())
		arrayTableMust(t, debug.Close())
		arrayTableMust(t, region.Close())
	})
	return output, input
}

func TestModelImplicantCompletedEvaluationUsesSourceVerify(t *testing.T) {
	for _, assertions := range []bool{true, false} {
		for _, limit := range []string{"max_steps", "max_memory"} {
			for _, array := range []bool{false, true} {
				for _, choice := range []string{"T", "C"} {
					t.Run(fmt.Sprintf("%s/array=%t/action=%s/assertions=%t", limit, array, choice, assertions), func(t *testing.T) {
						m, md, implicant, trace, tags := modelImplicantDiagnosticFixture(t)
						output, input := modelImplicantVerificationFixture(t, m, choice)
						arrayTableMust(t, diagnostic.BorrowProcessDebugContext().EnableAssertions(assertions))
						var pins []ast.ASTRef
						defer func() {
							for n := range pins {
								arrayTableMust(t, m.DelRef(&pins[n]))
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
						no := own(m.MkFalse())
						expression := own(m.MkNot(no))
						if array {
							boolean := own(m.MkBoolSort())
							sort := own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean))
							expression = own(m.MkConstArray(sort, no))
						}
						params, err := implicant.services.Context.NewValue()
						arrayTableMust(t, err)
						defer func() {
							arrayTableMust(t, implicant.reset())
							implicant.services.Params = nil
							arrayTableMust(t, implicant.services.Context.Del(&params))
						}()
						steps := limit
						arrayTableMust(t, implicant.services.Context.SetUIntName(&params, &steps, 0))
						implicant.services.Params = &params
						expectedMessage := messages.MaxSteps
						if limit == "max_memory" {
							expectedMessage = messages.MaxMemory
							block, err := implicant.services.Memory.Allocate(16)
							arrayTableMust(t, err)
							defer func() { arrayTableMust(t, implicant.services.Memory.Deallocate(&block)) }()
						}
						tags.Enable("model_evaluator")
						tags.Enable("pdr")
						before := m.Stats()
						result, err := implicant.EvalExpr(md, expression)
						nullSortRead := ast.ASTSourceDebug && choice == "C" && array && assertions
						continued := ast.ASTSourceDebug && choice == "C" && !nullSortRead
						if continued {
							arrayTableMust(t, err)
							if result == (ast.ASTRef{}) {
								t.Fatal("source continue lost its owning null result")
							}
							h, err := m.RefGet(result)
							arrayTableMust(t, err)
							if !h.IsNull() {
								t.Fatal("failed model::eval published a value", h)
							}
							arrayTableMust(t, m.DelRef(&result))
						} else {
							if err == nil || result != (ast.ASTRef{}) || rewriter.IsRewriterException(err) {
								t.Fatal("source failure boundary published output or leaked a caught exception", err, result)
							}
							if nullSortRead {
								if !errors.Is(err, ast.ErrASTHandle) {
									t.Fatal("null sort read lost its provider-domain guard", err)
								}
							} else if ast.ASTSourceDebug {
								var source z3exception.DefaultException
								if !errors.As(err, &source) {
									t.Fatal("source throw action lost its exception", err)
								}
							} else if !errors.Is(err, diagnostic.ErrDebugNativeUnreachable) {
								t.Fatal("release VERIFY lost its source domain failure", err)
							}
						}
						bytes, readErr := os.ReadFile(output.Name())
						arrayTableMust(t, readErr)
						if !strings.HasPrefix(string(bytes), "Failed to verify: m_model->eval(e, result, true)\n") {
							t.Fatal("missing source VERIFY diagnostic", string(bytes))
						}
						if ast.ASTSourceDebug && !strings.Contains(string(bytes), "File: src/model/model_implicant.cpp\nLine: 892\n") {
							t.Fatal("wrong verification source location", string(bytes))
						}
						state, err := input.State()
						arrayTableMust(t, err)
						expectedPosition := uint64(0)
						if ast.ASTSourceDebug {
							expectedPosition = 1
						}
						if state.Position != expectedPosition {
							t.Fatal("wrong debugger input consumption", state)
						}
						if tags.ShouldTrace("model_evaluator") && strings.Count(trace.String(), "[model_evaluator] eval src/model/model.cpp:88 ---------\n"+expectedMessage+"\n") != 1 {
							t.Fatal("missing model::eval exception catch trace", trace.String())
						}
						if nullSortRead && (strings.Contains(trace.String(), "no translation: null\n") || strings.Contains(string(bytes), "Line: 478\n")) {
							t.Fatal("invalid source condition read continued into assertion/action body", trace.String(), string(bytes))
						}
						if continued && array && tags.ShouldTrace("pdr") && !strings.Contains(trace.String(), "no translation: null\n") {
							t.Fatal("continued null array did not follow source extraction", trace.String())
						}
						after := m.Stats()
						if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
							t.Fatal("VERIFY failure/continue leaked ownership", after, before)
						}
						if implicant.model != nil || implicant.evaluator != nil || len(implicant.pendingEvaluators) != 0 || len(implicant.pendingArrays) != 0 {
							t.Fatal("verification left pending owner state")
						}
						implicant.services.Params = nil
						retry, err := implicant.EvalExpr(md, expression)
						arrayTableMust(t, err)
						h, err := m.RefGet(retry)
						arrayTableMust(t, err)
						if array {
							if h != expression {
								t.Fatal("array retry returned wrong value", h, expression)
							}
						} else if !m.IsTrue(h) {
							t.Fatal("Boolean retry did not evaluate true")
						}
						arrayTableMust(t, m.DelRef(&retry))
					})
				}
			}
		}
	}
}
