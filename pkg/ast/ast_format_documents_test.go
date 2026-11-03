package ast

import (
	"errors"
	"testing"
)

func TestASTFormatDocumentsPublishRawRootAndBorrowInputs(t *testing.T) {
	m := astTestManager(t)
	borrowed, err := m.FormatString("borrowed")
	astTestMust(t, err)
	formats, err := m.GetFormatManager()
	astTestMust(t, err)
	astTestPin(t, formats, borrowed)
	counts := func() [4]uint32 {
		s := formats.Stats()
		return [4]uint32{s.LiveNodes, s.LiveInfos, s.LiveParameters, s.LiveReferences}
	}
	before := counts()
	interrupted := errors.New("interrupted after child documents")
	produce := func(fail bool) (result ASTHandle, resultErr error) {
		documents, cause := newASTFormatDocuments(m)
		if cause != nil {
			return ASTHandle{}, cause
		}
		defer documents.finish(&result, &resultErr)
		child, cause := documents.own(m.FormatString("generated-child"))
		if cause != nil {
			return ASTHandle{}, cause
		}
		// Interned outputs can repeat without conflating their lexical owners.
		_, cause = documents.own(m.FormatString("generated-child"))
		if cause != nil {
			return ASTHandle{}, cause
		}
		parent, cause := documents.own(m.FormatCompose([]ASTHandle{borrowed, child}))
		if cause != nil {
			return ASTHandle{}, cause
		}
		if fail {
			return ASTHandle{}, interrupted
		}
		return parent, nil
	}
	result, err := produce(true)
	if !result.IsNull() || !errors.Is(err, interrupted) || counts() != before {
		t.Fatal("failed producer lost a document owner", result, err, before, counts())
	}
	result, err = produce(false)
	astTestMust(t, err)
	if refs, cause := formats.GetRefCount(result); cause != nil || refs != 0 {
		t.Fatal("formatter changed its raw RC0 result contract", refs, cause)
	}
	root, err := formats.NewRef(result)
	astTestMust(t, err)
	astTestMust(t, formats.DelRef(&root))
	if counts() != before {
		t.Fatal("successful producer left lexical references", before, counts())
	}
	if refs, cause := formats.GetRefCount(borrowed); cause != nil || refs != 1 {
		t.Fatal("formatter consumed its borrowed input", refs, cause)
	}
	passThrough := func() (result ASTHandle, resultErr error) {
		documents, cause := newASTFormatDocuments(m)
		if cause != nil {
			return ASTHandle{}, cause
		}
		defer documents.finish(&result, &resultErr)
		return borrowed, nil
	}
	result, err = passThrough()
	astTestMust(t, err)
	if result != borrowed || counts() != before {
		t.Fatal("passing through a borrowed result changed ownership")
	}
}
