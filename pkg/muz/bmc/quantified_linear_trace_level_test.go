package bmc_test

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"math/big"
	"os"
	"strings"
	"testing"
)

// Corrected C++ accepts a fact at any level and verifies a positive level
// before following a predecessor.
func TestQuantifiedLinearTraceFactLevel(t *testing.T) {
	for _, tc := range []struct {
		name          string
		level         int64
		wantErr       bool
		falseQuery    bool
		falseSelector bool
		noIndex       bool
		nonFact       bool
	}{{"zero", 0, false, false, false, false, false}, {"positive", 1, false, false, false, false, false}, {"false query debug assertion", 0, false, true, false, false, false}, {"missing selector debug assertion", 0, false, false, true, false, false}, {"non-numeral index verify", 0, false, false, false, true, false}, {"zero-level predecessor verify", 0, false, false, false, false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx, err := cmd_context.NewStandaloneContext()
			must(err)
			defer func() { must(ctx.Close()) }()
			m, err := ctx.Manager()
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("fact-level")
			must(err)
			predicate, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			head, err := m.MkApp(predicate, nil)
			must(err)
			rm, err := base.NewRuleManager(m)
			must(err)
			defer func() { must(rm.Close()) }()
			must(rm.RegisterPredicate(predicate))
			set, err := base.NewRuleSet(rm)
			must(err)
			defer func() { must(set.Close()) }()
			var tails []ast.ASTHandle
			if tc.nonFact {
				tails = append(tails, head)
			}
			rule, err := rm.NewRule(head, tails, nil, "fact")
			must(err)
			must(set.Add(rule))
			must(rule.Close())
			md, err := model.NewModel(m)
			must(err)
			defer func() { must(md.Close()) }()
			indexSort, err := m.BitVectorSort(4)
			must(err)
			tName, err := m.Symbols.Intern("T")
			must(err)
			index, err := m.MkConst(tName, indexSort)
			must(err)
			tDecl, err := m.GetDecl(index)
			must(err)
			level, err := m.BitVectorNumeralRat(big.NewRat(tc.level, 1), 4)
			must(err)
			if !tc.noIndex {
				must(md.Core().RegisterConstant(tDecl, level))
			}
			selectorName, err := m.Symbols.Intern("fact-level#0")
			must(err)
			selector, err := m.MkFuncDecl(selectorName, []ast.ASTHandle{indexSort}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			truth, err := m.MkTrue()
			must(err)
			fi, err := model.NewFunctionInterpretation(m, 1)
			must(err)
			selectorValue := truth
			if tc.falseSelector {
				selectorValue, err = m.MkFalse()
				must(err)
			}
			must(fi.SetElse(selectorValue))
			must(md.Core().RegisterFunction(selector, fi))
			queryName, err := m.Symbols.Intern("fact-level#")
			must(err)
			queryDecl, err := m.MkFuncDecl(queryName, []ast.ASTHandle{indexSort}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			queryInterpretation, err := model.NewFunctionInterpretation(m, 1)
			must(err)
			queryValue := truth
			if tc.falseQuery {
				queryValue, err = m.MkFalse()
				must(err)
			}
			must(queryInterpretation.SetElse(queryValue))
			must(md.Core().RegisterFunction(queryDecl, queryInterpretation))
			var diagnosticFile *os.File
			if ((tc.falseQuery || tc.falseSelector) && diagnostic.DebugBuild) || tc.noIndex || tc.nonFact {
				region, e := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: m.Budget()}})
				must(e)
				debug, e := diagnostic.NewDebugContext(region, m.Budget())
				must(e)
				var streams *diagnostic.MemoryStandardStreams
				if tc.noIndex || tc.nonFact {
					diagnosticFile, e = os.CreateTemp(t.TempDir(), "qlinear-verify")
					must(e)
					streams, e = diagnostic.NewMemoryStandardStreams(diagnostic.MemoryStandardStreamsConfig{Budget: m.Budget(), Stdout: diagnosticFile, Stderr: diagnosticFile})
					must(e)
					must(debug.BindAssertionStreams(streams))
				}
				previous, e := diagnostic.BindProcessDebugContext(debug)
				must(e)
				defer func() {
					_, e := diagnostic.BindProcessDebugContext(previous)
					must(e)
					must(debug.Close())
					if streams != nil {
						must(streams.Close())
						must(diagnosticFile.Close())
					}
					must(region.Close())
				}()
			}
			services, err := ctx.ModelEvaluatorConfig()
			must(err)
			compiler, err := bmc.NewCompiler(m)
			must(err)
			defer func() { must(compiler.Close()) }()
			trace, err := compiler.QuantifiedLinearTrace(set, predicate, 4, md.Core(), services)
			if tc.noIndex || tc.nonFact {
				if diagnostic.DebugBuild {
					if !errors.Is(err, diagnostic.ErrDebugAssertion) {
						t.Fatalf("missing debug VERIFY failure: %v", err)
					}
				} else if !errors.Is(err, diagnostic.ErrDebugNativeUnreachable) {
					t.Fatalf("missing release VERIFY guard: %v", err)
				}
				output, e := os.ReadFile(diagnosticFile.Name())
				must(e)
				message := "Failed to verify: m_bv.is_numeral(vl, num, bv_size)"
				if tc.nonFact {
					message = "Failed to verify: level > 0"
				}
				if !strings.Contains(string(output), message) {
					t.Fatalf("missing VERIFY output: %q", output)
				}
			} else if (tc.falseQuery || tc.falseSelector) && diagnostic.DebugBuild {
				if !errors.Is(err, diagnostic.ErrDebugAssertion) {
					t.Fatalf("missing source assertion: %v", err)
				}
			} else if tc.falseSelector {
				if err == nil {
					t.Fatal("missing selector unexpectedly succeeded")
				}
			} else if tc.wantErr {
				if !errors.Is(err, ast.ErrASTDomain) {
					t.Fatalf("positive-level fact: got %v, want AST domain failure", err)
				}
			} else {
				must(err)
				if len(trace) != 1 {
					t.Fatalf("zero-level fact: got %d rules", len(trace))
				}
			}
			for _, r := range trace {
				must(r.Close())
			}
		})
	}
}
