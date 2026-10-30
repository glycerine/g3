package base_test

import "testing"

func TestDatalogContextBackgroundSnapshotOwnsReferences(t *testing.T) {
	f := newDatalogBackgroundFixture(t)
	ruleMust(t, f.context.AssertExpression(f.first))
	ruleMust(t, f.context.AssertExpression(f.second))
	snapshot, err := f.context.BackgroundAssertions()
	ruleMust(t, err)
	ruleMust(t, f.context.Close())
	for i, reference := range snapshot {
		handle, getErr := f.manager.RefGet(reference)
		if getErr != nil {
			t.Fatal("snapshot lost ownership", i, getErr)
		}
		if i == 0 && !f.manager.IsTrue(handle) || i == 1 && !f.manager.IsFalse(handle) {
			t.Fatal("snapshot order", i)
		}
		ruleMust(t, f.manager.DelRef(&snapshot[i]))
	}
}
