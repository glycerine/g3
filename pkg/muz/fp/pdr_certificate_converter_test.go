package fp

import (
	"bytes"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestPDRCertificateConverterUsesCurrentLedgerAndTransfersModel(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e, m := f.engine, f.manager
	converter := pdrCertificateModelConverter{engine: e}
	truth, err := m.MkTrue()
	fragmentMust(t, err)
	falsity, err := m.MkFalse()
	fragmentMust(t, err)
	trueRef, err := m.NewRef(truth)
	fragmentMust(t, err)
	falseRef, err := m.NewRef(falsity)
	fragmentMust(t, err)
	// Install after bridge construction, as the actual preprocessing path does.
	e.pdrConversions = []hornModelConversion{
		{kind: hornConvertCone, cone: transforms.ConeModelStep{{Declaration: f.declarations["u"], Formula: trueRef}}},
		{kind: hornConvertCone, cone: transforms.ConeModelStep{{Declaration: f.declarations["u"], Formula: falseRef}}},
	}
	source, err := model.NewModel(m)
	fragmentMust(t, err)
	fragmentMust(t, source.Core().RegisterConstant(f.declarations["p"], truth))
	result, err := converter.ConvertCertificateModel(source)
	fragmentMust(t, err)
	if result != source {
		t.Fatal("live converters unexpectedly replaced model owner")
	}
	value, err := result.Core().ConstantInterpretation(f.declarations["u"])
	fragmentMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("certificate did not reverse-replay current converter ledger")
	}
	value, err = result.Core().ConstantInterpretation(f.declarations["p"])
	fragmentMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("conversion changed existing invariant")
	}
	// Published model owns its interpretations independently of the converter
	// ledger and of engine disposal, just like the source model_ref.
	e.pdrConversions = nil // This synthetic ledger borrows the test-owned roots.
	fragmentMust(t, m.DelRef(&trueRef))
	fragmentMust(t, m.DelRef(&falseRef))
	fragmentMust(t, e.Close())
	value, err = result.Core().ConstantInterpretation(f.declarations["u"])
	fragmentMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("model depended on retired converter ledger")
	}
	fragmentMust(t, result.Close())
}

func TestPDRInternalCertificateAppliesLiveSliceConverter(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e, m := f.engine, f.manager
	// Model conversions need m_mc, which a prior context::query installs.
	InstallContextModelConverter(e)
	for _, key := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear"} {
		fragmentMust(t, e.globals.SourceSetName(key, "false"))
	}
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.xform.slice", "true"))
	zero, err := m.ArithmeticIntegerNumeralText("0")
	fragmentMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	integer, err := m.GetSort(zero)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("certificate-original-P")
	fragmentMust(t, err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, e.RegisterPredicate(p))
	x, err := m.MkVar(0, integer)
	fragmentMust(t, err)
	fact, err := m.MkApp(p, []ast.ASTHandle{x, zero})
	fragmentMust(t, err)
	fragmentMust(t, e.addRuleEager(fact, "certificate-fact"))
	tail, err := m.MkApp(p, []ast.ASTHandle{x, one})
	fragmentMust(t, err)
	rule, err := m.MkImplies(tail, f.applications["u"])
	fragmentMust(t, err)
	fragmentMust(t, e.addRuleEager(rule, "certificate-query"))
	status, err := e.QueryPredicate(f.declarations["u"])
	fragmentMust(t, err)
	if status != lbool.False {
		t.Fatal("query", status)
	}
	if e.pdrQueryPredicate(p) == p {
		t.Fatal("fixture did not exercise slicing")
	}
	var output bytes.Buffer
	fragmentMust(t, e.pdr.DisplayCertificate(&output, e.pdrQueryPredicate(f.declarations["u"]), false))
	text := output.String()
	if !strings.Contains(text, "certificate-original-P") || strings.Contains(text, "!slice!") {
		t.Fatalf("internal certificate did not reconstruct original predicate: %s", text)
	}
	var again bytes.Buffer
	fragmentMust(t, e.pdr.DisplayCertificate(&again, e.pdrQueryPredicate(f.declarations["u"]), false))
	if again.String() != text {
		t.Fatal("converter consumed borrowed preprocessing ledger")
	}
}
