package base_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestDatalogContextBackgroundAssertionFailuresDoNotPublish(t *testing.T) {
	f := newDatalogBackgroundFixture(t)
	arithmetic, err := f.manager.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := f.manager.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	nonBoolean, err := f.manager.MkVar(0, integer)
	ruleMust(t, err)
	if err = f.context.AssertExpression(nonBoolean); err == nil {
		t.Fatal("non-Boolean assertion accepted")
	}
	if count, e := f.context.BackgroundAssertionCount(); e != nil || count != 0 {
		t.Fatal("failed assertion published", count, e)
	}
	if _, err = f.context.BackgroundAssertion(-1); err == nil {
		t.Fatal("negative assertion index accepted")
	}
	ruleMust(t, f.context.Close())
	if _, err = f.context.BackgroundFormula(); !errors.Is(err, base.ErrDatalogContextOwner) {
		t.Fatal("closed context accepted", err)
	}
}
