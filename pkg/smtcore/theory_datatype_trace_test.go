package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func smtDatatypeTraceRecord(tag, function string, line int, body string) string {
	return fmt.Sprintf("-------- [%s] %s src/smt/theory_datatype.cpp:%d ---------\n%s------------------------------------------------\n", tag, function, line, body)
}

func TestSMTDatatypeEmptyRecognizerTracePrecedesDomainGuard(t *testing.T) {
	_, dt, x, _, _ := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	variable := s.context.enodes.get(x).TheoryVar(dt.FamilyID())

	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("datatype")
	dt.trace = tags
	err = dt.propagateRecognizer(variable, nil)
	if !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("empty recognizer guard", err)
	}
	if tags.ShouldTrace("datatype") {
		want := "-------- [datatype] propagate_recognizer src/smt/theory_datatype.cpp:750 ---------\nLogical context:\n"
		if !strings.HasPrefix(output.String(), want) {
			t.Fatal("empty recognizer context trace", output.String(), want)
		}
	} else if output.Len() != 0 {
		t.Fatal("native empty-recognizer trace was not erased")
	}

	dt.trace = diagnostic.NewTraceTagsWithWriter(smtPBDisplayFailure{})
	dt.trace.Enable("datatype")
	err = dt.propagateRecognizer(variable, nil)
	if !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("sticky trace stream masked empty recognizer guard", err)
	}
}

func TestSMTDatatypeExhaustionTraceUsesTriggerAndOrderedEvidence(t *testing.T) {
	for _, first := range []int{2, 0} {
		t.Run(fmt.Sprint(first), func(t *testing.T) {
			m, dt, x, sort, constructors := smtDatatypeSplitFixture(t)
			s, err := dt.State()
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "exhaustion-y"), sort)
			astTestMust(t, err)
			astTestMust(t, s.Internalize(y, false))
			var literals []SMTLiteral
			var expressions []ast.ASTHandle
			for i, constructor := range constructors {
				value := x
				if i != 0 {
					value = y
				}
				literal, expression, err := dt.recognizerLiteral(constructor, value)
				astTestMust(t, err)
				astTestMust(t, s.markRelevant(expression))
				literals = append(literals, literal)
				expressions = append(expressions, expression)
			}
			astTestMust(t, s.addEquality(s.context.enodes.get(x), s.context.enodes.get(y), NewSMTEqualityAxiom()))
			_, err = s.PropagateCore()
			astTestMust(t, err)
			if s.Inconsistent() {
				t.Fatal("unassigned recognizers conflicted during merge")
			}
			for attempt := 0; attempt < 2; attempt++ {
				astTestMust(t, s.PushScope())
				var trace bytes.Buffer
				tags := diagnostic.NewTraceTagsWithWriter(&trace)
				tags.Enable("datatype_conflict")
				dt.trace = tags
				order := []int{first}
				for i := range literals {
					if i != first {
						order = append(order, i)
					}
				}
				// All assignments precede theory callbacks. The first callback,
				// rather than the last assignment, triggers exhaustion.
				for _, i := range order {
					astTestMust(t, s.AssignLiteral(literals[i].Negated(), NewSMTBooleanAxiom()))
				}
				_, err = s.PropagateCore()
				astTestMust(t, err)
				conflict, found := s.Conflict()
				if !found {
					t.Fatal("exhausted recognizers did not conflict")
				}
				owner, _ := conflict.Justification()
				extended, ok := owner.(*SMTExtTheoryConflictJustification)
				if !ok || len(extended.Literals) != 3 || len(extended.Equalities) == 0 {
					t.Fatal("exhaustion lost literal or merged-class equality evidence", owner)
				}
				want := ""
				if tags.ShouldTrace("datatype_conflict") {
					var body bytes.Buffer
					astTestMust(t, dt.displayDatatypeExpression(&body, expressions[first]))
					body.WriteByte('\n')
					for i, literal := range extended.Literals {
						if literal != literals[i].Negated() {
							t.Fatal("exhaustion evidence not in constructor declaration order")
						}
						astTestMust(t, literal.Display(&body, m, s.boolExpr))
						body.WriteByte('\n')
					}
					for _, pair := range extended.Equalities {
						if pair.First.root != pair.Second.root || pair.First == pair.Second {
							t.Fatal("exhaustion used invalid class equality")
						}
						astTestMust(t, dt.displayDatatypeExpression(&body, pair.First.owner))
						body.WriteString(" = ")
						astTestMust(t, dt.displayDatatypeExpression(&body, pair.Second.owner))
						body.WriteByte('\n')
					}
					want = smtDatatypeTraceRecord("datatype_conflict", "propagate_recognizer", 780, body.String())
				}
				if trace.String() != want {
					t.Fatal("exhaustion trigger/evidence trace", trace.String(), want)
				}
				astTestMust(t, s.PopScopes(1))
				if s.Inconsistent() {
					t.Fatal("exhaustion conflict survived pop")
				}
			}
		})
	}
}

func TestSMTDatatypeEqualityTraceSourceBranchesBeforeQueuedMerge(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, guard := range []string{"null", "true", "undefined"} {
			t.Run(fmt.Sprint(proofs, "/", guard), func(t *testing.T) {
				m, dt, x, sort, _ := smtDatatypeSplitFixture(t)
				if proofs {
					astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
				}
				s, err := dt.State()
				astTestMust(t, err)
				y, err := m.MkConst(astTestSymbol(t, m, "datatype-trace-y"), sort)
				astTestMust(t, err)
				astTestMust(t, s.Internalize(y, false))
				p := smtRelevancyBool(t, m, "datatype-trace-guard")
				astTestMust(t, s.Internalize(p, true))
				antecedent, err := s.Literal(p)
				astTestMust(t, err)
				if guard == "null" {
					antecedent = SMTNullLiteral
				} else if guard == "true" {
					astTestMust(t, s.AssignLiteral(antecedent, NewSMTBooleanAxiom()))
				}
				var output bytes.Buffer
				tags := diagnostic.NewTraceTagsWithWriter(&output)
				tags.Enable("datatype")
				tags.Enable("datatype_detail")
				dt.trace = tags
				astTestMust(t, dt.equalityAxiom(x, y, antecedent))
				// context::mark_as_relevant propagates at once, so the axiom's
				// equality reaches x and y and relevant_eh traces them, as in
				// the source; this test checks the assert_eq_axiom records.
				trimmed := smtDatatypeDropRecords(output.String(), "relevant_eh")
				output.Reset()
				output.WriteString(trimmed)
				if proofs || !tags.ShouldTrace("datatype") {
					if output.Len() != 0 {
						t.Fatal("proof branch or native erasure evaluated datatype trace operands", output.String())
					}
					return
				}
				want := smtDatatypeTraceRecord("datatype", "assert_eq_axiom", 100, "adding axiom:\nincremental-x\n=\ndatatype-trace-y\n")
				if guard != "true" {
					if output.String() != want {
						t.Fatal("source equality trace branch", output.String(), want)
					}
					return
				}
				left, right := s.context.enodes.get(x), s.context.enodes.get(y)
				lid, err := left.OwnerID()
				astTestMust(t, err)
				rid, err := right.OwnerID()
				astTestMust(t, err)
				want += smtDatatypeTraceRecord("datatype", "assert_eq_axiom", 116, fmt.Sprintf("assigning... #%d #%d\nv%d v%d\n", lid, rid, left.TheoryVar(dt.FamilyID()), right.TheoryVar(dt.FamilyID())))
				want += "-------- [datatype_detail] assert_eq_axiom src/smt/theory_datatype.cpp:118 ---------\nTheory datatype:\n"
				if !strings.HasPrefix(output.String(), want) || left.root == right.root {
					t.Fatal("source pre-merge record order", output.String(), want)
				}
			})
		}
	}
}

func TestSMTDatatypeTraceEntryOrderAndIndependentDetailTag(t *testing.T) {
	m, dt, x, sort, constructors := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	recognizer, err := m.DatatypeConstructorRecognizer(constructors[0])
	astTestMust(t, err)
	atom, err := m.MkApp(recognizer, []ast.ASTHandle{x})
	astTestMust(t, err)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("datatype")
	dt.trace = tags
	astTestMust(t, s.Internalize(atom, true))
	if tags.ShouldTrace("datatype") {
		atomEntry := strings.Index(output.String(), "[datatype] internalize_atom src/smt/theory_datatype.cpp:258")
		termEntry := strings.Index(output.String(), "[datatype] internalize_term src/smt/theory_datatype.cpp:263")
		if atomEntry < 0 || termEntry <= atomEntry {
			t.Fatal("source datatype entry records missing or reordered", output.String())
		}
	} else if output.Len() != 0 {
		t.Fatal("native entry trace was not erased")
	}
	y, err := m.MkConst(astTestSymbol(t, m, "detail-trace-y"), sort)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(y, false))
	literal, err := s.Literal(atom)
	astTestMust(t, err)
	astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
	output.Reset()
	tags.Disable("datatype")
	tags.Enable("datatype_detail")
	astTestMust(t, dt.equalityAxiom(x, y, literal))
	if tags.ShouldTrace("datatype_detail") {
		if !strings.HasPrefix(output.String(), "-------- [datatype_detail] assert_eq_axiom src/smt/theory_datatype.cpp:118") || strings.Contains(output.String(), "[datatype]") {
			t.Fatal("detail tag was coupled to datatype tag", output.String())
		}
	} else if output.Len() != 0 {
		t.Fatal("native detail trace was not erased")
	}
}

func TestSMTDatatypeTraceBadStreamPreservesQueuedEquality(t *testing.T) {
	m, dt, x, sort, _ := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "bad-stream-datatype-y"), sort)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(y, false))
	tags := diagnostic.NewTraceTagsWithWriter(smtPBDisplayFailure{})
	tags.Enable("datatype")
	dt.trace = tags
	astTestMust(t, dt.equalityAxiom(x, y, SMTNullLiteral))
	if s.context.enodes.get(x).root == s.context.enodes.get(y).root {
		t.Fatal("bad stream changed source deferred equality")
	}
	_, err = s.PropagateCore()
	astTestMust(t, err)
	if s.context.enodes.get(x).root != s.context.enodes.get(y).root {
		t.Fatal("sticky ostream failure stopped equality publication")
	}
	var output bytes.Buffer
	dt.trace = diagnostic.NewTraceTagsWithWriter(&output)
	astTestMust(t, dt.traceDatatypeExpression("internalize_term", 263, "erased:\n", ast.ASTHandle{}))
	if output.Len() != 0 {
		t.Fatal("disabled trace evaluated operands")
	}
}

func TestSMTDatatypeOccursTraceLeafExitAndCachedSkip(t *testing.T) {
	_, dt, x, _, _ := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	node := s.context.enodes.get(x)
	id, err := node.OwnerID()
	astTestMust(t, err)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("datatype")
	dt.trace = tags
	check := smtDatatypeOccursCheck{theory: dt, onStack: make(map[*SMTEnode]bool), cycleFree: make(map[*SMTEnode]bool), parent: make(map[*SMTEnode]*SMTEnode)}
	cycle, err := check.run(node)
	astTestMust(t, err)
	if cycle || !check.cycleFree[node.root] || s.Inconsistent() {
		t.Fatal("leaf diagnostic changed occurs-check traversal")
	}
	entry := smtDatatypeTraceRecord("datatype", "occurs_check", 524, fmt.Sprintf("occurs check: #%d incremental-x\n", id))
	want := entry + smtDatatypeTraceRecord("datatype", "occurs_check", 538, fmt.Sprintf("occurs check loop: #%d incremental-x enter\n", id)) + smtDatatypeTraceRecord("datatype", "occurs_check", 538, fmt.Sprintf("occurs check loop: #%d incremental-x exit\n", id))
	if tags.ShouldTrace("datatype") {
		if output.String() != want {
			t.Fatal("source leaf/exit diagnostics", output.String(), want)
		}
	} else if output.Len() != 0 {
		t.Fatal("native leaf diagnostic was not erased")
	}
	output.Reset()
	cycle, err = check.run(node)
	astTestMust(t, err)
	if cycle || tags.ShouldTrace("datatype") && output.String() != entry {
		t.Fatal("cycle-free skip evaluated loop diagnostic", cycle, output.String())
	}
	dt.trace = diagnostic.NewTraceTagsWithWriter(smtPBDisplayFailure{})
	dt.trace.Enable("datatype")
	check = smtDatatypeOccursCheck{theory: dt, onStack: make(map[*SMTEnode]bool), cycleFree: make(map[*SMTEnode]bool), parent: make(map[*SMTEnode]*SMTEnode)}
	cycle, err = check.run(node)
	astTestMust(t, err)
	if cycle || !check.cycleFree[node.root] {
		t.Fatal("sticky ostream failure stopped cycle-free publication")
	}
}

func TestSMTDatatypeSplitTracePrecedesRecognizerInternalization(t *testing.T) {
	_, dt, x, sort, _ := smtDatatypeSplitFixture(t)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("datatype")
	tags.Enable("datatype_bug")
	dt.trace = tags
	before := dt.statistics.splits
	astTestMust(t, dt.split(x, sort))
	if dt.statistics.splits != before+1 {
		t.Fatal("split trace changed source counter")
	}
	if tags.ShouldTrace("datatype") {
		last := -1
		for _, site := range []string{"[datatype_bug] mk_split src/smt/theory_datatype.cpp:829", "[datatype] mk_split src/smt/theory_datatype.cpp:879", "[datatype] internalize_atom src/smt/theory_datatype.cpp:258", "[datatype] internalize_term src/smt/theory_datatype.cpp:263", "[datatype] add_recognizer src/smt/theory_datatype.cpp:712"} {
			i := strings.Index(output.String(), site)
			if i <= last {
				t.Fatal("source split/registration trace order", site, output.String())
			}
			last = i
		}
	} else if output.Len() != 0 {
		t.Fatal("native split trace was not erased")
	}
	output.Reset()
	astTestMust(t, dt.split(x, sort))
	if tags.ShouldTrace("datatype_bug") {
		want := smtDatatypeTraceRecord("datatype_bug", "mk_split", 829, "non_rec_c: incremental-b\n")
		if output.String() != want {
			t.Fatal("waiting on existing recognizer emitted a creation record", output.String(), want)
		}
	}
	if dt.statistics.splits != before+2 {
		t.Fatal("waiting split did not retain its source call count")
	}
}

func TestSMTDatatypeRecognizerAssignmentTraceAndConstructorPrefix(t *testing.T) {
	for _, truth := range []bool{false, true} {
		t.Run(fmt.Sprint(truth), func(t *testing.T) {
			_, dt, x, _, constructors := smtDatatypeSplitFixture(t)
			s, err := dt.State()
			astTestMust(t, err)
			literal, expression, err := dt.recognizerLiteral(constructors[1], x)
			astTestMust(t, err)
			astTestMust(t, s.markRelevant(expression))
			assigned := literal
			if !truth {
				assigned = assigned.Negated()
			}
			astTestMust(t, s.AssignLiteral(assigned, NewSMTBooleanAxiom()))
			var output bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&output)
			tags.Enable("datatype")
			tags.Enable("datatype_bug")
			dt.trace = tags
			before := dt.statistics.constructorAxioms
			astTestMust(t, s.propagateAssignedAtom(assigned))
			wantDelta := uint32(0)
			bit := 0
			if truth {
				wantDelta, bit = 1, 1
			}
			if dt.statistics.constructorAxioms != before+wantDelta {
				t.Fatal("assignment diagnostic changed constructor publication")
			}
			if tags.ShouldTrace("datatype") {
				id, err := s.context.enodes.get(expression).OwnerID()
				astTestMust(t, err)
				prefix := fmt.Sprintf("-------- [datatype] assign_eh src/smt/theory_datatype.cpp:367 ---------\nassigning recognizer: #%d is_true: %d\n", id, bit)
				if !strings.HasPrefix(output.String(), prefix) {
					t.Fatal("source recognizer assignment record", output.String(), prefix)
				}
				constructor := strings.Index(output.String(), "[datatype_bug] assert_is_constructor_axiom src/smt/theory_datatype.cpp:129")
				if truth && constructor <= 0 || !truth && constructor >= 0 {
					t.Fatal("constructor trace branch/order", output.String())
				}
			} else if output.Len() != 0 {
				t.Fatal("native assignment trace was not erased")
			}
			_, err = s.PropagateCore()
			astTestMust(t, err)
			if s.Inconsistent() {
				t.Fatal("diagnostic changed consistent recognizer assignment")
			}
		})
	}
}

func TestSMTDatatypeMergeTracePrecedesImportedRecognizer(t *testing.T) {
	m, dt, x, sort, constructors := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "merge-trace-y"), sort)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(y, false))
	_, first, err := dt.recognizerLiteral(constructors[0], x)
	astTestMust(t, err)
	_, second, err := dt.recognizerLiteral(constructors[1], y)
	astTestMust(t, err)
	for _, h := range []ast.ASTHandle{first, second} {
		astTestMust(t, s.markRelevant(h))
	}
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("datatype")
	dt.trace = tags
	left, right := s.context.enodes.get(x), s.context.enodes.get(y)
	astTestMust(t, s.addEquality(left, right, NewSMTEqualityAxiom()))
	astTestMust(t, s.PropagateTheoryEvents())
	root, data, err := dt.rootData(left.TheoryVar(dt.FamilyID()))
	astTestMust(t, err)
	child := left.TheoryVar(dt.FamilyID())
	imported := first
	if uint32(child) == root {
		child, imported = right.TheoryVar(dt.FamilyID()), second
	}
	if len(data.recognizers) != 3 || data.recognizers[0] == nil || data.recognizers[1] == nil {
		t.Fatal("merge diagnostic changed recognizer import")
	}
	if tags.ShouldTrace("datatype") {
		id, err := s.context.enodes.get(imported).OwnerID()
		astTestMust(t, err)
		want := smtDatatypeTraceRecord("datatype", "merge_eh", 663, fmt.Sprintf("merging v%d v%d\n", root, child)) + smtDatatypeTraceRecord("datatype", "add_recognizer", 712, fmt.Sprintf("adding recognizer to v%d rec: #%d val: l_undef\n", root, id))
		if output.String() != want {
			t.Fatal("source merge/import records", output.String(), want)
		}
	} else if output.Len() != 0 {
		t.Fatal("native merge trace was not erased")
	}
}

func TestSMTDatatypeSortConstraintTraceIncludesAttachedSkip(t *testing.T) {
	_, dt, x, sort, _ := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	node := s.context.enodes.get(x)
	id, err := node.OwnerID()
	astTestMust(t, err)
	before := dt.NumVariables()
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("datatype")
	tags.Enable("datatype_bug")
	dt.trace = tags
	astTestMust(t, dt.ApplySortConstraint(node, sort))
	if dt.NumVariables() != before {
		t.Fatal("sort constraint diagnostic duplicated an attached variable")
	}
	bug := smtDatatypeTraceRecord("datatype_bug", "apply_sort_cnstr", 344, "apply_sort_cnstr:\nincremental-x\n")
	if tags.ShouldTrace("datatype") {
		want := smtDatatypeTraceRecord("datatype", "apply_sort_cnstr", 343, fmt.Sprintf("apply_sort_cnstr: #%d\n", id)) + bug
		if output.String() != want {
			t.Fatal("source sort constraint records", output.String(), want)
		}
	} else if output.Len() != 0 {
		t.Fatal("native sort constraint diagnostic was not erased")
	}
	output.Reset()
	tags.Disable("datatype")
	astTestMust(t, dt.ApplySortConstraint(node, sort))
	if tags.ShouldTrace("datatype_bug") && output.String() != bug {
		t.Fatal("sort-constraint bug tag was coupled to datatype tag", output.String())
	}
}

func TestSMTDatatypeRelevanceTracePrecedesRegistrationAndDuplicateSkip(t *testing.T) {
	m, dt, x, _, constructors := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("datatype")
	dt.trace = tags
	astTestMust(t, dt.Relevant(x))
	if tags.ShouldTrace("datatype") {
		want := smtDatatypeTraceRecord("datatype", "relevant_eh", 396, "relevant_eh: incremental-x\n")
		if output.String() != want {
			t.Fatal("non-recognizer relevance entry was skipped", output.String(), want)
		}
	} else if output.Len() != 0 {
		t.Fatal("native relevance entry was not erased")
	}
	_, expression, err := dt.recognizerLiteral(constructors[1], x)
	astTestMust(t, err)
	output.Reset()
	astTestMust(t, s.markRelevant(expression))
	if tags.ShouldTrace("datatype") {
		last := -1
		for _, site := range []string{"[datatype] relevant_eh src/smt/theory_datatype.cpp:396", "[datatype] relevant_eh src/smt/theory_datatype.cpp:400", "[datatype] add_recognizer src/smt/theory_datatype.cpp:712"} {
			i := strings.Index(output.String(), site)
			if i <= last {
				t.Fatal("source relevance/registration record order", site, output.String())
			}
			last = i
		}
		id, err := m.GetASTID(expression)
		astTestMust(t, err)
		if !strings.Contains(output.String(), fmt.Sprintf("relevant_eh: #%d\n", id)) {
			t.Fatal("relevance record lost source AST identity", output.String())
		}
	}
	output.Reset()
	astTestMust(t, dt.Relevant(expression))
	if strings.Contains(output.String(), "[datatype] add_recognizer") {
		t.Fatal("already watched recognizer was registered twice")
	}
	_, data, err := dt.rootData(s.context.enodes.get(x).TheoryVar(dt.FamilyID()))
	astTestMust(t, err)
	if len(data.recognizers) != 3 || data.recognizers[1] != s.context.enodes.get(expression) {
		t.Fatal("relevance diagnostic changed source watch slot")
	}
}

// smtDatatypeDropRecords removes the datatype TRACE records of function.
func smtDatatypeDropRecords(trace, function string) string {
	var out strings.Builder
	header := "-------- [datatype] " + function + " "
	footer := "------------------------------------------------\n"
	for len(trace) > 0 {
		if strings.HasPrefix(trace, header) {
			end := strings.Index(trace, footer)
			if end < 0 {
				break
			}
			trace = trace[end+len(footer):]
			continue
		}
		next := strings.Index(trace, "\n")
		if next < 0 {
			out.WriteString(trace)
			break
		}
		out.WriteString(trace[:next+1])
		trace = trace[next+1:]
	}
	return out.String()
}
