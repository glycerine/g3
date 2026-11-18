package smtcore

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func bvModelSourceFixture(t *testing.T) (*ast.ASTManager, *SMTInternalizerState, *SMTBitVectorTheory, *SMTModelGenerator) {
	t.Helper()
	m, cfg := smtModelGeneratorFixture(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	family, err := m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	bv, err := NewSMTBitVectorTheory(family)
	astTestMust(t, err)
	astTestMust(t, s.RegisterTheory(bv))
	g, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	astTestMust(t, g.SetContext(s.context))
	astTestMust(t, g.InitializeModel(cfg))
	t.Cleanup(func() { astTestMust(t, g.Close()) })
	return m, s, bv, g
}

func TestSMTBVModelValueNativePrefixAndDebugFixedness(t *testing.T) {
	for _, tc := range []struct {
		name  string
		bits  []lbool.LBool
		want  string
		fixed bool
	}{
		{"first-undefined", []lbool.LBool{lbool.Undef, lbool.True, lbool.True, lbool.True}, "0", false},
		{"middle-undefined", []lbool.LBool{lbool.True, lbool.Undef, lbool.True, lbool.True}, "1", false},
		{"last-undefined", []lbool.LBool{lbool.True, lbool.False, lbool.True, lbool.Undef}, "5", false},
		{"fully-fixed", []lbool.LBool{lbool.True, lbool.False, lbool.False, lbool.True}, "9", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, report := newLRASourceAssertionContext(t)
			m, s, bv, g := bvModelSourceFixture(t)
			node, v := bvFixedSymbolic(t, m, s, bv, "native-model-"+tc.name, 4)
			for index, value := range tc.bits {
				if value == lbool.Undef {
					continue
				}
				literal := bv.bitLiterals[v][index]
				if value == lbool.False {
					literal = literal.Negated()
				}
				astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
			}
			astTestMust(t, bv.InitializeModel(g))
			prefix, fixed, err := bv.testFixedLiteralValue(v)
			astTestMust(t, err)
			if prefix != tc.want || fixed != tc.fixed {
				t.Fatal("source get_fixed_value prefix", prefix, fixed, tc.want, tc.fixed)
			}
			before := m.Stats()
			if smtcoreSourceDebug && !tc.fixed {
				m.Budget().FailAfter(0)
			}
			procedure, err := bv.ModelValue(node, g)
			m.Budget().FailAfter(-1)
			if smtcoreSourceDebug && !tc.fixed {
				bvInvariantFailure(t, err, report, 1601, "r")
				if procedure != nil || m.Stats() != before {
					t.Fatal("source fixedness assertion must precede factory construction", err, procedure)
				}
				return
			}
			astTestMust(t, err)
			value, err := procedure.MakeValue(g, nil)
			astTestMust(t, err)
			text, yes, err := m.BitVectorNumeralText(value)
			astTestMust(t, err)
			width, err := m.BitVectorSize(value)
			astTestMust(t, err)
			if !yes || text != tc.want || width != 4 || procedure.IsFresh() || len(procedure.Dependencies()) != 0 {
				t.Fatal("native BV expression-wrapper model", text, width, tc.want)
			}
		})
	}
}

func TestSMTBVModelValueSignedLiteralsAndFactoryPipeline(t *testing.T) {
	m, s, bv, g := bvModelSourceFixture(t)
	p, q := smtRelevancyBool(t, m, "model-signed-p"), smtRelevancyBool(t, m, "model-signed-q")
	notQ, err := m.MkNot(q)
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	node, _ := bvNativeWord(t, m, s, bv, p, notQ, truth, falsity)
	for _, expr := range []ast.ASTHandle{p, notQ} {
		literal, err := s.Literal(expr)
		astTestMust(t, err)
		astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
	}
	astTestMust(t, s.context.SetRelevant(node.owner, true))
	// The real generator initializes the factory and invokes the registered
	// theory's ModelValue callback, then pins the returned expression.
	astTestMust(t, g.BuildStateValues(s))
	value, err := g.Value(node)
	astTestMust(t, err)
	text, yes, err := m.BitVectorNumeralText(value)
	astTestMust(t, err)
	width, err := m.BitVectorSize(value)
	astTestMust(t, err)
	if !yes || text != "7" || width != 4 {
		t.Fatal("native signed and constant model bits", text, width)
	}
}

func TestSMTBVModelValueRuntimeAssertionsAndActions(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, action := range []string{"T", "C"} {
			t.Run(fmt.Sprint(enabled, "/", action), func(t *testing.T) {
				d, _ := newLRASourceAssertionContext(t)
				report := bvDebugActionInput(t, d, strings.Repeat(action, 8))
				astTestMust(t, d.EnableAssertions(enabled))
				m, s, bv, g := bvModelSourceFixture(t)
				node, v := bvFixedSymbolic(t, m, s, bv, "runtime-model-prefix", 4)
				for _, index := range []int{0, 2, 3} {
					astTestMust(t, s.AssignLiteral(bv.bitLiterals[v][index], NewSMTBooleanAxiom()))
				}
				// The first undefined bit ends native accumulation before the high bits.
				astTestMust(t, bv.InitializeModel(g))
				before := m.Stats()
				procedure, err := bv.ModelValue(node, g)
				if diagnostic.DebugBuild && enabled && action == "T" {
					bvInvariantFailure(t, err, report, 1601, "r")
					if procedure != nil || m.Stats() != before {
						t.Fatal("throw touched the model factory")
					}
					return
				}
				astTestMust(t, err)
				value, err := procedure.MakeValue(g, nil)
				astTestMust(t, err)
				text, yes, err := m.BitVectorNumeralText(value)
				astTestMust(t, err)
				width, err := m.BitVectorSize(value)
				astTestMust(t, err)
				if !yes || text != "1" || width != 4 || procedure.IsFresh() || len(procedure.Dependencies()) != 0 {
					t.Fatal("source continued prefix/factory", text, width)
				}
				output, err := os.ReadFile(report)
				astTestMust(t, err)
				if diagnostic.DebugBuild && enabled {
					if !strings.Contains(string(output), "Line: 1601\nr\n") || strings.Count(string(output), "ASSERTION VIOLATION") != 1 {
						t.Fatal("source continue notification", string(output))
					}
				} else if len(output) != 0 {
					t.Fatal("disabled/native model assertion executed", string(output))
				}
			})
		}
	}
}

func TestSMTBVModelValueVariableAssertionPrecedesValueAndFactory(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			d, report := newLRASourceAssertionContext(t)
			astTestMust(t, d.EnableAssertions(enabled))
			m, s, bv, g := bvModelSourceFixture(t)
			node, _ := bvFixedSymbolic(t, m, s, bv, "model-missing-variable", 4)
			astTestMust(t, bv.InitializeModel(g))
			variable := node.TheoryVar(bv.FamilyID())
			astTestMust(t, node.DeleteTheoryVar(bv.FamilyID()))
			defer func() { astTestMust(t, node.AddTheoryVar(bv.FamilyID(), variable)) }()
			before := m.Stats()
			m.Budget().FailAfter(0)
			procedure, err := bv.ModelValue(node, g)
			m.Budget().FailAfter(-1)
			if diagnostic.DebugBuild && enabled {
				bvInvariantFailure(t, err, report, 1596, "v != null_theory_var")
			} else if !errors.Is(err, ast.ErrASTDomain) {
				t.Fatal("invalid slot Go domain guard", err)
			}
			if procedure != nil || m.Stats() != before {
				t.Fatal("variable assertion read a value or factory")
			}
		})
	}
}
