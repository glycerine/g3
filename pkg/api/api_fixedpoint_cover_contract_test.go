package api

import (
	"strings"
	"testing"
)

func TestSMTContextFixedpointPrequeryCoverSlicingContract(t *testing.T) {
	c, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, c.Close()) }()
	astTestMust(t, c.SetFixedpointEngine("pdr"))
	declaration, err := c.DeclareRelation("api-prequery-cover", nil)
	astTestMust(t, err)
	m, err := c.Manager()
	astTestMust(t, err)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	err = c.AddFixedpointCover(2, declaration, falsity)
	if err == nil || !strings.Contains(err.Error(), "Covers are incompatible with slicing") {
		t.Fatal("default slicing did not reject a pre-query cover", err)
	}
	astTestMust(t, c.SetFixedpointBool(FixedpointTransformSlice, false))
	astTestMust(t, c.AddFixedpointCover(2, declaration, falsity))
	levels, err := c.FixedpointNumLevels(declaration)
	astTestMust(t, err)
	if levels != 3 {
		t.Fatal("disabled slicing did not permit the cover", levels)
	}
	cover, err := c.FixedpointCoverDelta(2, declaration)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&cover)) }()
	handle, err := m.RefGet(cover)
	astTestMust(t, err)
	if !m.IsFalse(handle) {
		t.Fatal("public pre-query cover changed")
	}
}
