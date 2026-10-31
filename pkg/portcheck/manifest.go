// Package portcheck inventories preserved reference material for maintainers.
// Ordinary semantic builds and tests do not read the reference checkout.
package portcheck

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const SchemaVersion = 2

type Manifest struct {
	Schema   int    `json:"schema"`
	Baseline string `json:"baseline"`
	Units    []Unit `json:"units"`
	APIs     []API  `json:"api_operations"`
}

// GoTarget is the primary implementation. GoTargets records explicit additional
// implementation or test files when a source method spans Go packages.
type Method struct {
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	GoTarget  string   `json:"go_target,omitempty"`
	GoTargets []string `json:"go_targets,omitempty"`
	Tests     []string `json:"tests,omitempty"`
	Evidence  string   `json:"evidence,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}
type Line struct {
	Number int    `json:"line"`
	Text   string `json:"text"`
}
type Unit struct {
	Path             string   `json:"path"`
	Component        string   `json:"component"`
	Role             string   `json:"role"`
	Status           string   `json:"status"`
	GoTargets        []string `json:"go_targets"`
	Tests            []string `json:"tests"`
	Conditionals     []Line   `json:"conditionals,omitempty"`
	Assertions       []Line   `json:"assertions,omitempty"`
	Registrations    []Line   `json:"registrations,omitempty"`
	MethodsAudited   bool     `json:"methods_audited"`
	OwnershipAudited bool     `json:"ownership_audited"`
	Methods          []Method `json:"methods"`
	// Ownership accepts both the original prose strings and newer structured
	// evidence records. Validation currently requires only the audited flag;
	// retaining RawMessage keeps the ledger lossless across both schema-2 forms.
	Ownership  []json.RawMessage `json:"ownership"`
	Deviations []string          `json:"deviations"`
}
type API struct {
	Name      string   `json:"name"`
	Sources   []string `json:"sources"`
	Status    string   `json:"status"`
	GoTarget  string   `json:"go_target,omitempty"`
	GoTargets []string `json:"go_targets,omitempty"`
	Tests     []string `json:"tests,omitempty"`
}

var apiPattern = regexp.MustCompile(`\bZ3_API\s+(Z3_[A-Za-z0-9_]+)\s*\(`)
var assertionPattern = regexp.MustCompile(`\b(?:SASSERT|CASSERT|ASSERT|ENSURE|VERIFY)\s*\(`)
var registrationPattern = regexp.MustCompile(`\b(?:ADD_TACTIC|ADD_PROBE|ADD_SIMPLIFIER|REG_PARAMS|ADD_INITIALIZER|ADD_FINALIZER|add_lib|add_exe|add_dll|add_dot_net_dll|add_java_dll|z3_add_component)\s*\(`)

// Strip C/C++ comments while preserving newlines and string literals. This is
// lexical inventory, not preprocessing: inactive branches remain visible.
func uncomment(s string) string {
	b := []byte(s)
	out := append([]byte(nil), b...)
	for i := 0; i < len(b); {
		if b[i] == '"' || b[i] == '\'' {
			q := b[i]
			i++
			for i < len(b) {
				if b[i] == '\\' {
					i += 2
					continue
				}
				if b[i] == q {
					i++
					break
				}
				i++
			}
			continue
		}
		if i+1 < len(b) && b[i] == '/' && b[i+1] == '/' {
			for i < len(b) && b[i] != '\n' {
				out[i] = ' '
				i++
			}
			continue
		}
		if i+1 < len(b) && b[i] == '/' && b[i+1] == '*' {
			out[i] = ' '
			out[i+1] = ' '
			i += 2
			for i < len(b) {
				if i+1 < len(b) && b[i] == '*' && b[i+1] == '/' {
					out[i] = ' '
					out[i+1] = ' '
					i += 2
					break
				}
				if b[i] != '\n' {
					out[i] = ' '
				}
				i++
			}
			continue
		}
		i++
	}
	return string(out)
}
func sourceUnit(p string) bool {
	e := path.Ext(p)
	return strings.HasPrefix(p, "src/") && (e == ".cpp" || e == ".h" || e == ".hpp" || e == ".pyg")
}

func Inventory(root string) (Manifest, error) {
	m := Manifest{Schema: SchemaVersion, Baseline: "patched Z3 4.7.1.0", Units: []Unit{}, APIs: []API{}}
	paths := []string{}
	for _, dir := range []string{"src", "scripts", "examples", "cmake"} {
		base := filepath.Join(root, dir)
		if _, err := os.Stat(base); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "__pycache__" || d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("reference symlink needs explicit classification: %s", p)
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if path.Ext(rel) == ".pyc" {
				return nil
			}
			paths = append(paths, rel)
			return nil
		})
		if err != nil {
			return m, err
		}
	}
	for _, p := range []string{"CMakeLists.txt", "LICENSE.txt"} {
		if _, err := os.Stat(filepath.Join(root, p)); err == nil {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	apis := map[string][]string{}
	for _, p := range paths {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			return m, err
		}
		parts := strings.Split(p, "/")
		component := parts[0]
		role := "reference_tool"
		if parts[0] == "src" {
			component = parts[1]
			role = "production"
			if !sourceUnit(p) {
				role = "reference_support"
			}
			if component == "test" {
				role = "test"
			}
			if path.Ext(p) == ".pyg" {
				role = "generated_input"
			}
			if path.Base(p) == "CMakeLists.txt" {
				role = "build_specification"
			}
		}
		u := Unit{Path: p, Component: component, Role: role, Status: "unported", GoTargets: []string{}, Tests: []string{}, Methods: []Method{}, Ownership: []json.RawMessage{}, Deviations: []string{}}
		clean := uncomment(string(b))
		for i, line := range strings.Split(clean, "\n") {
			t := strings.TrimSpace(line)
			l := Line{i + 1, t}
			if strings.HasPrefix(t, "#if") || strings.HasPrefix(t, "#else") || strings.HasPrefix(t, "#elif") || strings.HasPrefix(t, "#endif") {
				u.Conditionals = append(u.Conditionals, l)
			}
			if assertionPattern.MatchString(line) {
				u.Assertions = append(u.Assertions, l)
			}
			if registrationPattern.MatchString(line) {
				u.Registrations = append(u.Registrations, l)
			}
		}
		if strings.HasPrefix(p, "src/api/") {
			for _, match := range apiPattern.FindAllStringSubmatch(clean, -1) {
				name := match[1]
				seen := false
				for _, s := range apis[name] {
					if s == p {
						seen = true
					}
				}
				if !seen {
					apis[name] = append(apis[name], p)
				}
			}
		}
		m.Units = append(m.Units, u)
	}
	names := []string{}
	for name := range apis {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, n := range names {
		m.APIs = append(m.APIs, API{Name: n, Sources: apis[n], Status: "unported"})
	}
	return m, nil
}
func validPath(p string) bool {
	return p != "" && !strings.HasPrefix(p, "/") && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../") && !strings.Contains(p, "\\")
}
func validStatus(s string) bool {
	return s == "unported" || s == "partial" || s == "ported" || s == "platform_replacement" || s == "reference_only" || s == "dead_deleted"
}
func Validate(m Manifest) error {
	if m.Schema != SchemaVersion {
		return fmt.Errorf("unsupported schema %d", m.Schema)
	}
	seen := map[string]bool{}
	for _, u := range m.Units {
		if !validPath(u.Path) || seen[u.Path] {
			return fmt.Errorf("invalid/duplicate unit %q", u.Path)
		}
		seen[u.Path] = true
		if !validStatus(u.Status) {
			return fmt.Errorf("invalid status for %s", u.Path)
		}
		if u.Status == "ported" {
			if len(u.GoTargets) == 0 || len(u.Tests) == 0 || !u.MethodsAudited || !u.OwnershipAudited || len(u.Methods) == 0 {
				return fmt.Errorf("missing completion evidence for %s", u.Path)
			}
			for _, method := range u.Methods {
				switch method.Status {
				case "ported", "platform_replacement":
					if (method.GoTarget != "" || len(method.GoTargets) != 0) && (len(method.Tests) != 0 || method.Evidence != "" || len(u.Tests) != 0) {
						continue
					}
				case "reference_only", "dead_deleted", "dead":
					continue
				}
				return fmt.Errorf("unfinished method %s in %s", method.Name, u.Path)
			}
		}
		if (u.Status == "platform_replacement" || u.Status == "reference_only" || u.Status == "dead_deleted") && len(u.Deviations) == 0 {
			return fmt.Errorf("unexplained classification for %s", u.Path)
		}
		for _, p := range u.GoTargets {
			if !validPath(p) {
				return fmt.Errorf("invalid Go target %q", p)
			}
		}
		for _, method := range u.Methods {
			for _, p := range append([]string{method.GoTarget}, method.GoTargets...) {
				if p != "" && !validPath(p) {
					return fmt.Errorf("invalid Go target %q for method %s in %s", p, method.Name, u.Path)
				}
			}
		}
	}
	apiSeen := map[string]bool{}
	for _, a := range m.APIs {
		if a.Name == "" || apiSeen[a.Name] || !validStatus(a.Status) {
			return fmt.Errorf("invalid/duplicate API %q", a.Name)
		}
		apiSeen[a.Name] = true
		for _, p := range a.Sources {
			if !seen[p] {
				return fmt.Errorf("API %s source %s absent", a.Name, p)
			}
		}
		for _, p := range append([]string{a.GoTarget}, a.GoTargets...) {
			if p != "" && !validPath(p) {
				return fmt.Errorf("invalid Go target %q for API %s", p, a.Name)
			}
		}
		if a.Status == "ported" && (a.GoTarget == "" || len(a.Tests) == 0) {
			return fmt.Errorf("missing API completion evidence %s", a.Name)
		}
	}
	return nil
}

// Compare ignores progress metadata while requiring exact reference coverage.
func Compare(ledger, current Manifest) error {
	if err := Validate(ledger); err != nil {
		return err
	}
	if len(ledger.Units) != len(current.Units) || len(ledger.APIs) != len(current.APIs) {
		return fmt.Errorf("source/API inventory count changed")
	}
	units := map[string]Unit{}
	for _, u := range ledger.Units {
		units[u.Path] = u
	}
	for _, u := range current.Units {
		_, ok := units[u.Path]
		if !ok {
			return fmt.Errorf("missing reference %s", u.Path)
		}
	}
	apis := map[string]API{}
	for _, a := range ledger.APIs {
		apis[a.Name] = a
	}
	for _, a := range current.APIs {
		old, ok := apis[a.Name]
		if !ok || strings.Join(old.Sources, "\n") != strings.Join(a.Sources, "\n") {
			return fmt.Errorf("missing or changed API %s", a.Name)
		}
	}
	return nil
}
func Load(p string) (Manifest, error) {
	var m Manifest
	b, err := os.ReadFile(p)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}
func Save(p string, m Manifest) error {
	if err := Validate(m); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0644)
}
