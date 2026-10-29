package core

import "testing"

func TestPBPreprocessTranslationIsFresh(t *testing.T) {
	f := newPBPreprocessFixture(t)
	translated, e := f.runner.Translate(f.m)
	simplifyMust(t, e)
	copy := translated.(*PBPreprocessTactic)
	simplifyMust(t, copy.Cleanup())
	simplifyMust(t, copy.Close())
}
