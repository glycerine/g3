package portcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutableEvidenceRejectsNonRunnableNames(t *testing.T) {
	for _, item := range []struct{ name, kind string }{
		{"Testlowercase", "T"}, {"Testéclair", "T"},
		{"Fuzzlowercase", "F"}, {"Benchmarklowercase", "B"},
	} {
		t.Run(item.name, func(t *testing.T) {
			root := t.TempDir()
			source(t, root, "api.go", "package fixture\nfunc One() {}\n")
			source(t, root, "api_test.go", fmt.Sprintf("package fixture\nimport \"testing\"\nfunc %s(t *testing.%s) {}\n", item.name, item.kind))
			m := Manifest{Units: []Unit{{Path: "src/util/fixture.cpp", GoTargets: []string{"api.go"}, Tests: []string{item.name}}}}
			if e := VerifyEvidence(m, root); e == nil || !strings.Contains(e.Error(), "missing runnable test declaration "+item.name) {
				t.Fatal("Go ignores a lowercase rune after the entry-point prefix", e)
			}
		})
	}
}

func TestExecutableEvidenceAcceptsRunnablePrefixes(t *testing.T) {
	for _, item := range []struct{ name, kind string }{
		{"Test", "T"}, {"TestUppercase", "T"}, {"TestÉclair", "T"}, {"Test_", "T"},
		{"Fuzz0", "F"}, {"BenchmarkUppercase", "B"},
	} {
		t.Run(item.name, func(t *testing.T) {
			root := t.TempDir()
			source(t, root, "api.go", "package fixture\nfunc One() {}\n")
			source(t, root, "api_test.go", fmt.Sprintf("package fixture\nimport alias \"testing\"\nfunc %s(t *alias.%s) {}\n", item.name, item.kind))
			m := Manifest{Units: []Unit{{Path: "src/util/fixture.cpp", GoTargets: []string{"api.go"}, Tests: []string{item.name}}}}
			if e := VerifyEvidence(m, root); e != nil {
				t.Fatal("real Go entry-point spelling and testing import alias", e)
			}
		})
	}
}

func TestExecutableEvidenceRefreshesChangedTests(t *testing.T) {
	root := t.TempDir()
	source(t, root, "api.go", "package fixture\nfunc One() {}\n")
	source(t, root, "api_test.go", "package fixture\nimport \"testing\"\nfunc TestOriginal(t *testing.T) {}\n")
	m := Manifest{Units: []Unit{{Path: "src/util/fixture.cpp", GoTargets: []string{"api.go"}, Tests: []string{"TestOriginal"}}}}
	if e := VerifyEvidence(m, root); e != nil {
		t.Fatal(e)
	}
	source(t, root, "api_test.go", "package fixture\nimport \"testing\"\nfunc TestReplacement(t *testing.T) {}\n")
	if e := VerifyEvidence(m, root); e == nil {
		t.Fatal("prior invocation accepted a removed declaration")
	}
	m.Units[0].Tests = []string{"TestReplacement"}
	if e := VerifyEvidence(m, root); e != nil {
		t.Fatal("new invocation must see actual current Go files", e)
	}
}

func TestExecutableEvidenceAcceptsEmbeddedPackageAssetTarget(t *testing.T) {
	root := t.TempDir()
	source(t, root, "pattern/database.smt2", "(assert true)\n")
	source(t, root, "pattern/database_test.go", "package pattern\nimport \"testing\"\nfunc TestDatabase(t *testing.T) {}\n")
	m := Manifest{Units: []Unit{{Path: "src/ast/pattern/database.h", GoTargets: []string{"pattern/database.smt2"}, Tests: []string{"TestDatabase"}}}}
	if err := VerifyEvidence(m, root); err != nil {
		t.Fatal(err)
	}
}

func TestExecutableEvidenceKeepsMethodPackageScope(t *testing.T) {
	root := t.TempDir()
	for _, item := range []struct{ dir, name string }{{"first", "TestFirst"}, {"second", "TestSecond"}} {
		source(t, root, item.dir+"/api.go", "package fixture\nfunc One() {}\n")
		source(t, root, item.dir+"/api_test.go", fmt.Sprintf("package fixture\nimport \"testing\"\nfunc %s(t *testing.T) {}\n", item.name))
	}
	m := Manifest{Units: []Unit{{Path: "src/util/fixture.cpp", GoTargets: []string{"first/api.go", "second/api.go"}, Tests: []string{"TestFirst", "TestSecond"}, Methods: []Method{{Name: "Second", GoTarget: "second/api.go", Tests: []string{"TestFirst"}}}}}}
	if e := VerifyEvidence(m, root); e == nil || !strings.Contains(e.Error(), "src/util/fixture.cpp::Second missing runnable test declaration TestFirst") {
		t.Fatal("unit-wide evidence must not spill into a different method package", e)
	}
	m.Units[0].Methods[0].Tests = []string{"TestSecond"}
	if e := VerifyEvidence(m, root); e != nil {
		t.Fatal(e)
	}
}

func TestExecutableEvidenceExplicitSplitMethodTargets(t *testing.T) {
	root := t.TempDir()
	for _, item := range []struct{ dir, name string }{{"leaf", "TestLeaf"}, {"caller", "TestCaller"}} {
		source(t, root, item.dir+"/impl.go", "package fixture\nfunc One() {}\n")
		source(t, root, item.dir+"/impl_test.go", fmt.Sprintf("package fixture\nimport \"testing\"\nfunc %s(t *testing.T) {}\n", item.name))
	}
	method := Method{Name: "Split", GoTarget: "leaf/impl.go", Tests: []string{"TestLeaf", "TestCaller"}}
	m := Manifest{Units: []Unit{{Path: "src/util/fixture.cpp", GoTargets: []string{"leaf/impl.go", "caller/impl.go"}, Methods: []Method{method}}}}
	if err := VerifyEvidence(m, root); err == nil {
		t.Fatal("unit-wide targets must not implicitly extend method evidence")
	}
	m.Units[0].Methods[0].GoTargets = []string{"caller/impl.go"}
	if err := VerifyEvidence(m, root); err != nil {
		t.Fatal("explicit split implementation should include both packages", err)
	}
	if err := os.Remove(filepath.Join(root, "caller/impl_test.go")); err != nil {
		t.Fatal(err)
	}
	if err := VerifyEvidence(m, root); err == nil {
		t.Fatal("explicit package target must not conceal removed tests")
	}
}

func BenchmarkExecutableEvidenceSharedDirectory(b *testing.B) {
	root := b.TempDir()
	if e := os.WriteFile(filepath.Join(root, "api.go"), []byte("package fixture\nfunc One() {}\n"), 0600); e != nil {
		b.Fatal(e)
	}
	for i := 0; i < 64; i++ {
		body := fmt.Sprintf("package fixture\nimport \"testing\"\nfunc TestCase%d(t *testing.T) {}\n", i)
		if e := os.WriteFile(filepath.Join(root, fmt.Sprintf("case%d_test.go", i)), []byte(body), 0600); e != nil {
			b.Fatal(e)
		}
	}
	m := Manifest{}
	for i := 0; i < 100; i++ {
		u := Unit{Path: fmt.Sprintf("src/util/fixture%d.cpp", i), GoTargets: []string{"api.go"}, Tests: []string{"TestCase0"}}
		for j := 0; j < 5; j++ {
			u.Methods = append(u.Methods, Method{Name: fmt.Sprint(j), GoTarget: "api.go", Tests: []string{"TestCase1"}})
		}
		m.Units = append(m.Units, u)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if e := VerifyEvidence(m, root); e != nil {
			b.Fatal(e)
		}
	}
}

func TestExecutableAPIEvidenceExplicitSplitTargets(t *testing.T) {
	root := t.TempDir()
	source(t, root, "api/impl.go", "package api\n")
	source(t, root, "api/impl_test.go", "package api\nimport \"testing\"\nfunc TestAPI(t *testing.T) {}\n")
	source(t, root, "cli/integration_test.go", "package cli\nimport \"testing\"\nfunc TestCLI(t *testing.T) {}\n")
	m := Manifest{APIs: []API{{Name: "Z3_fixture", GoTarget: "api/impl.go", Tests: []string{"TestAPI", "TestCLI/case"}}}}
	if err := VerifyEvidence(m, root); err == nil {
		t.Fatal("unlinked cross-package test accepted")
	}
	m.APIs[0].GoTargets = []string{"cli/integration_test.go"}
	if err := VerifyEvidence(m, root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "cli/integration_test.go")); err != nil {
		t.Fatal(err)
	}
	if err := VerifyEvidence(m, root); err == nil {
		t.Fatal("removed API evidence accepted")
	}
}
