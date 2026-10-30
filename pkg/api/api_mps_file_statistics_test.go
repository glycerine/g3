package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSMTContextRunMPSFileWithStatistics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.mps")
	astTestMust(t, os.WriteFile(path, []byte(apiOneVariableMPS("L", 5, "")), 0600))
	f := newSMTAPIFixture(t, "ALL", nil)
	result, err := f.c.RunMPSFile(path, &MPSRunOptions{PrintStatistics: true, ReportFrequency: 1})
	astTestMust(t, err)
	if !strings.Contains(result.Output, "status is OPTIMAL") || !strings.Contains(result.Output, "iterations") {
		t.Fatal("MPS statistics output", result.Output)
	}
}
