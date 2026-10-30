package base_test

import "testing"

func TestDatalogContextBackgroundAssertionsFollowScopes(t *testing.T) {
	f := newDatalogBackgroundFixture(t)
	ruleMust(t, f.context.AssertExpression(f.first))
	ruleMust(t, f.context.Push())
	ruleMust(t, f.context.AssertExpression(f.second))
	ruleMust(t, f.context.Push())
	ruleMust(t, f.context.AssertExpression(f.first))
	ruleMust(t, f.context.Pop(1))
	if count, err := f.context.BackgroundAssertionCount(); err != nil || count != 2 {
		t.Fatal("inner pop", count, err)
	}
	ruleMust(t, f.context.Pop(1))
	if count, err := f.context.BackgroundAssertionCount(); err != nil || count != 1 {
		t.Fatal("outer pop", count, err)
	}
	assertion, err := f.context.BackgroundAssertion(0)
	if err != nil || assertion != f.first {
		t.Fatal("outer assertion", assertion, err)
	}
}
