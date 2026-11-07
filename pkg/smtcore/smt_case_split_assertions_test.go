package smtcore

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uheap "github.com/glycerine/g3/pkg/util/heap"
)

func checkCaseSplitAssertion(t *testing.T, report string, err error, enabled bool, fallback error, line int, condition string) {
	t.Helper()
	checkLRASourceAssertionResult(t, err, enabled, fallback)
	if enabled && diagnostic.DebugBuild {
		data, e := os.ReadFile(report)
		astTestMust(t, e)
		for _, want := range []string{"src/smt/smt_case_split_queue.cpp", fmt.Sprint(line), condition} {
			if !strings.Contains(string(data), want) {
				t.Fatal("wrong source assertion", want, string(data))
			}
		}
	}
}

func TestSMTCaseSplitSourceCreationAssertionsBeforeHeapMutation(t *testing.T) {
	d, report := newLRASourceAssertionContext(t)
	for _, enabled := range []bool{false, true} {
		astTestMust(t, d.EnableAssertions(enabled))
		for _, strategy := range []uint32{0, 1, 4} {
			t.Run(fmt.Sprintf("%t/%d", enabled, strategy), func(t *testing.T) {
				m, s, _, _ := queueTraceFixture(t, strategy)
				line, condition := 85, "!m_queue.contains(v)"
				var err error
				if strategy == 4 {
					astTestMust(t, s.beginSearchPersistence(0, SMTParamsState{}))
					defer func() { astTestMust(t, s.endSearchPersistence()) }()
					err = s.relevanceActivityCreated(0)
					line, condition = 514, "v >= m_bs_num_bool_vars"
					checkCaseSplitAssertion(t, report, err, enabled, nil, line, condition)
					if s.relevanceActivity.delayed.Contains(0) {
						astTestMust(t, s.relevanceActivity.delayed.Erase(0))
					}
					return
				}
				if strategy == 1 {
					line = 174
				}
				before := m.Budget().Used()
				err = s.basicActivityCreated(0)
				checkCaseSplitAssertion(t, report, err, enabled, uheap.ErrValue, line, condition)
				if m.Budget().Used() != before {
					t.Fatal("duplicate creation allocated")
				}
				if strategy == 1 {
					astTestMust(t, s.delayedActivity.Insert(0))
					err = s.basicActivityCreated(0)
					checkCaseSplitAssertion(t, report, err, enabled, uheap.ErrValue, 173, "!m_delayed_queue.contains(v)")
				}
			})
		}
	}
}

func TestSMTCaseSplitSourceAdmissionAndCandidateAssertions(t *testing.T) {
	d, report := newLRASourceAssertionContext(t)
	for _, enabled := range []bool{false, true} {
		astTestMust(t, d.EnableAssertions(enabled))
		for _, strategy := range []uint32{3, 4, 5} {
			t.Run(fmt.Sprintf("%t/%d", enabled, strategy), func(t *testing.T) {
				m, s, _, q := queueTraceFixture(t, strategy)
				a := smtRelevancyBool(t, m, "assert-candidate-a")
				_, _, _, err := q.relevantCandidate(a)
				line := 411
				if strategy == 4 {
					line = 603
				} else if strategy == 5 {
					line = 867
				}
				checkCaseSplitAssertion(t, report, err, enabled, nil, line, "intern || is_or")
				astTestMust(t, s.Internalize(a, true))
				av, _ := s.BoolVar(a)
				s.boolVar.set(a, SMTNullBoolVar)
				err = s.admitRelevanceCaseSplit(a)
				s.boolVar.set(a, av)
				line = 343
				if strategy == 4 {
					line = 542
				} else if strategy == 5 {
					line = 775
				}
				checkCaseSplitAssertion(t, report, err, enabled, ast.ErrASTDomain, line, "var != null_bool_var")
			})
		}
	}
}

func TestSMTCaseSplitSourceScopeAssertionsAndRestorationOrder(t *testing.T) {
	d, report := newLRASourceAssertionContext(t)
	for _, enabled := range []bool{false, true} {
		astTestMust(t, d.EnableAssertions(enabled))
		for _, strategy := range []uint32{3, 4, 5} {
			t.Run(fmt.Sprintf("%t/%d", enabled, strategy), func(t *testing.T) {
				_, s, _, _ := queueTraceFixture(t, strategy)
				line := 391
				if strategy == 4 {
					line = 585
				} else if strategy == 5 {
					line = 831
				}
				err := s.PopScopes(1)
				checkCaseSplitAssertion(t, report, err, enabled, ast.ErrASTDomain, line, "num_scopes <= m_scopes.size()")
				astTestMust(t, s.PushScope())
				saved := s.scopes[0]
				line = 399
				switch strategy {
				case 3:
					s.scopes[0].relevanceFIFO.firstHead = saved.relevanceFIFO.firstLimit + 1
				case 4:
					s.scopes[0].relActHead = saved.relActLimit + 1
					line = 591
				case 5:
					s.scopes[0].goalQueue.head = saved.goalQueue.firstLimit + 1
					line = 859
				}
				err = s.PopScopes(1)
				actualHead := s.relevanceFIFO.firstHead
				if strategy == 4 {
					actualHead = s.relevanceActivity.head
				} else if strategy == 5 {
					actualHead = s.relevanceGoal.head
				}
				if actualHead != 1 {
					t.Fatal("head assertion ran before restoration", strategy, actualHead)
				}
				s.scopes[0] = saved
				checkCaseSplitAssertion(t, report, err, enabled, ast.ErrASTDomain, line, "m_head <= m_queue.size()")
				astTestMust(t, s.PopScopes(1))
			})
		}
	}
}

func TestSMTCaseSplitSourceGoalHeapAssertionsAndRecovery(t *testing.T) {
	d, report := newLRASourceAssertionContext(t)
	for _, enabled := range []bool{false, true} {
		astTestMust(t, d.EnableAssertions(enabled))
		for _, line := range []int{841, 850, 916, 918} {
			t.Run(fmt.Sprintf("%t/%d", enabled, line), func(t *testing.T) {
				m, s, p, q := queueTraceFixture(t, 5)
				f := &s.relevanceGoal
				a := smtRelevancyBool(t, m, "assert-secondary-a")
				astTestMust(t, s.Internalize(a, true))
				var err error
				fallback := error(nil)
				condition := ""
				switch line {
				case 916:
					astTestMust(t, f.heap.Reserve(1))
					astTestMust(t, f.heap.Insert(0))
					_, _, err = q.NextCaseSplit()
					fallback = ast.ErrASTDomain
					condition = "idx < m_queue2.size()"
				case 918:
					astTestMust(t, p.MarkRelevant(a))
					f.second[0].lastDecided = 0
					_, _, err = q.NextCaseSplit()
					condition = "e.m_last_decided == -1"
					f.second[0].lastDecided = -1
					if !f.heap.Contains(0) {
						astTestMust(t, f.heap.Insert(0))
					}
				case 841:
					astTestMust(t, s.PushScope())
					astTestMust(t, p.MarkRelevant(a))
					f.second[0].lastDecided = 0
					err = s.PopScopes(1)
					condition = "(m_queue2[i].m_last_decided == -1) == m_priority_queue2.contains(i)"
					if len(s.scopes) > 0 {
						f.second[0].lastDecided = -1
						astTestMust(t, s.PopScopes(1))
					}
				case 850:
					astTestMust(t, p.MarkRelevant(a))
					astTestMust(t, s.PushScope())
					f.second[0].lastDecided = 1
					err = s.PopScopes(1)
					fallback = uheap.ErrValue
					condition = "!m_priority_queue2.contains(i)"
					f.second[0].lastDecided = -1
					if len(s.scopes) > 0 {
						astTestMust(t, s.PopScopes(1))
					}
				}
				checkCaseSplitAssertion(t, report, err, enabled, fallback, line, condition)
			})
		}
	}
}

func TestSMTCaseSplitSourceTheoryRandomAssertionsAndErasedOperands(t *testing.T) {
	d, report := newLRASourceAssertionContext(t)
	for _, enabled := range []bool{false, true} {
		astTestMust(t, d.EnableAssertions(enabled))
		m, s, _, q := queueTraceFixture(t, 6)
		a := smtRelevancyBool(t, m, "assert-aware-a")
		astTestMust(t, s.Internalize(a, true))
		q.randomVariableFrequency = -1
		before := s.random
		_, _, err := q.NextCaseSplit()
		checkCaseSplitAssertion(t, report, err, enabled, nil, 1167, "threshold >= 0")
		if enabled && diagnostic.DebugBuild && s.random != before {
			t.Fatal("negative threshold assertion consumed RNG")
		}
		saved := s.boolExpr
		s.boolExpr = nil
		_, err = s.theoryAwareBranching.next(1)
		s.boolExpr = saved
		checkCaseSplitAssertion(t, report, err, enabled, ast.ErrASTDomain, 1169, "m_context.get_num_b_internalized() > 0")
	}
}
