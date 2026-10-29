package api

import "testing"

func TestAPIVersionQueries(t *testing.T) {
	version := Version()
	if version != (APIVersion{Major: 4, Minor: 7, Build: 1, Revision: 0}) {
		t.Fatal("version tuple", version)
	}
	if FullVersion() != "Z3 4.7.1.0" {
		t.Fatal("full version", FullVersion())
	}
}
