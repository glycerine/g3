package tab

// Source: src/muz/tab/tab_context.cpp imp::query and select_predicate.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Source ostream refusal sets badbit without interrupting later operands.
// Formatting and AST-construction errors still propagate from their callers.
type tabDiagnosticWriter struct{ stream *utext.NumericTextStream }

func (w tabDiagnosticWriter) Write(body []byte) (int, error) {
	_ = w.stream.AppendCString(string(body))
	return len(body), nil
}

func tabVerboseStream(minimum uint32) (tabDiagnosticWriter, bool, error) {
	enabled, err := diagnostic.ProcessVerboseEnabled(minimum)
	if err != nil || !enabled {
		return tabDiagnosticWriter{}, false, err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	return tabDiagnosticWriter{stream: stream}, err == nil, err
}

func (s *tabSearch) verboseInitialClause() error {
	out, enabled, err := tabVerboseStream(1)
	if err != nil || !enabled {
		return err
	}
	clause := s.stack[len(s.stack)-1]
	_, _ = fmt.Fprintf(out, "g%d ", clause.sequence)
	return clause.Display(out)
}

func (s *tabSearch) verboseSelectedPredicate(clause *Clause) error {
	out, enabled, err := tabVerboseStream(2)
	if err != nil || !enabled {
		return err
	}
	predicate, err := s.manager.RefGet(clause.predicates[clause.predicateIndex])
	if err != nil {
		return err
	}
	if err = writeMkPP(out, s.manager, predicate); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out)
	return nil
}

func (s *tabSearch) verboseEmptyClause(clause *Clause) error {
	out, enabled, err := tabVerboseStream(2)
	if err != nil || !enabled {
		return err
	}
	return clause.Display(out)
}

// The search progress lines below are AUDIT.md FUZZ-F5 ports, compiled only
// into g3trace builds (diagnostic.VerboseProgress).

// tabVerboseText is IF_VERBOSE(minimum, verbose_stream() << text).
func tabVerboseText(minimum uint32, text string) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	out, enabled, err := tabVerboseStream(minimum)
	if err != nil || !enabled {
		return err
	}
	_ = out.stream.AppendCString(text)
	return out.stream.Flush()
}

// tabVerboseDoubles is IF_VERBOSE(minimum, a << " " << b << "\n") with the
// stream's current double format.
func tabVerboseDoubles(minimum uint32, a, b float64) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	out, enabled, err := tabVerboseStream(minimum)
	if err != nil || !enabled {
		return err
	}
	_ = out.stream.AppendFloat64(a)
	_ = out.stream.AppendCString(" ")
	_ = out.stream.AppendFloat64(b)
	_ = out.stream.AppendCString("\n")
	return out.stream.Flush()
}

// tabVerboseScore is IF_VERBOSE(2, "score: " << mk_pp(p, m) << " " << score).
func tabVerboseScore(m *ast.ASTManager, p ast.ASTHandle, score float64) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	out, enabled, err := tabVerboseStream(2)
	if err != nil || !enabled {
		return err
	}
	_ = out.stream.AppendCString("score: ")
	if err = writeMkPP(out, m, p); err != nil {
		return err
	}
	_ = out.stream.AppendCString(" ")
	_ = out.stream.AppendFloat64(score)
	_ = out.stream.AppendCString("\n")
	return out.stream.Flush()
}

// verboseInstruction is run()'s IF_VERBOSE(2, m_instruction << "\n").
func verboseInstruction(name string) error { return tabVerboseText(2, name+"\n") }

// verboseApplyRule is apply_rule's IF_VERBOSE(1, display_rule(*clause);
// display_premise(*clause, "g" << next->get_seqno() << " ");
// display_clause(*next)). display_rule prints each rule once, as
// "r<next rule>: " and the rule clause.
func (s *tabSearch) verboseApplyRule(current *Clause, rule *Clause, next *Clause) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	out, enabled, err := tabVerboseStream(1)
	if err != nil || !enabled {
		return err
	}
	if s.displayedRules == nil {
		s.displayedRules = make(map[*Clause]bool)
	}
	if !s.displayedRules[rule] {
		s.displayedRules[rule] = true
		_, _ = fmt.Fprintf(out, "r%d: ", current.nextRule)
		if err = rule.Display(out); err != nil {
			return err
		}
	}
	predicate, err := s.manager.RefGet(current.predicates[current.predicateIndex])
	if err != nil {
		return err
	}
	decl, err := s.manager.GetDecl(predicate)
	if err != nil {
		return err
	}
	name, err := s.manager.GetASTName(decl)
	if err != nil {
		return err
	}
	text, err := s.manager.Symbols.String(name)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "g%d {g%d %s pos: %d rule: %d}\n", next.sequence, current.sequence, text, current.predicateIndex, current.nextRule)
	if err = next.Display(out); err != nil {
		return err
	}
	return out.stream.Flush()
}

// tabVerbosePair is IF_VERBOSE(2, label << mk_pp(s) << " " << mk_pp(t) << "\n").
func tabVerbosePair(m *ast.ASTManager, label string, s, t ast.ASTHandle) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	out, enabled, err := tabVerboseStream(2)
	if err != nil || !enabled {
		return err
	}
	_ = out.stream.AppendCString(label)
	if err = writeMkPP(out, m, s); err != nil {
		return err
	}
	_ = out.stream.AppendCString(" ")
	if err = writeMkPP(out, m, t); err != nil {
		return err
	}
	_ = out.stream.AppendCString("\n")
	return out.stream.Flush()
}

// tabVerboseSetupMatch is index::setup's IF_VERBOSE(2, "setup-match: " <<
// each m_preds[i] << " " << m_precond).
func tabVerboseSetupMatch(m *ast.ASTManager, preds []ast.ASTRef, precond ast.ASTRef) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	out, enabled, err := tabVerboseStream(2)
	if err != nil || !enabled {
		return err
	}
	_ = out.stream.AppendCString("setup-match: ")
	for _, ref := range preds {
		h, err := m.RefGet(ref)
		if err != nil {
			return err
		}
		if err = writeMkPP(out, m, h); err != nil {
			return err
		}
		_ = out.stream.AppendCString(" ")
	}
	h, err := m.RefGet(precond)
	if err != nil {
		return err
	}
	if err = writeMkPP(out, m, h); err != nil {
		return err
	}
	_ = out.stream.AppendCString("\n")
	return out.stream.Flush()
}

// tabVerboseTryMatch is match_rule's IF_VERBOSE(2, g.display(verbose_stream() << "try-match\n")).
func tabVerboseTryMatch(g *Clause) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	out, enabled, err := tabVerboseStream(2)
	if err != nil || !enabled {
		return err
	}
	_ = out.stream.AppendCString("try-match\n")
	if err = g.Display(out); err != nil {
		return err
	}
	return out.stream.Flush()
}

// tabVerboseMatchPredicate is match_predicates' IF_VERBOSE(2, index spaces,
// mk_pp(q) << " = " << mk_pp(p)).
func tabVerboseMatchPredicate(m *ast.ASTManager, index int, q, p ast.ASTHandle) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	out, enabled, err := tabVerboseStream(2)
	if err != nil || !enabled {
		return err
	}
	for j := 0; j < index; j++ {
		_ = out.stream.AppendCString(" ")
	}
	if err = writeMkPP(out, m, q); err != nil {
		return err
	}
	_ = out.stream.AppendCString(" = ")
	if err = writeMkPP(out, m, p); err != nil {
		return err
	}
	_ = out.stream.AppendCString("\n")
	return out.stream.Flush()
}

// verboseCertificate is display_certificate: mk_pp(get_answer()) at level 1.
func (s *tabSearch) verboseCertificate() (result error) {
	out, enabled, err := tabVerboseStream(1)
	if err != nil || !enabled {
		return err
	}
	answer, err := s.Proof()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, s.manager.DelRef(&answer)) }()
	if err = DisplayCertificate(s.manager, out, answer); err != nil {
		return err
	}
	return out.stream.Flush()
}

// verboseBodyInsts is display_body_insts: compose the substitutions from the
// root step down (dl_util apply_subst), apply them to clause.get_body() with
// var_subst(m, false), and print mk_pp of the result at level 1.
func (s *tabSearch) verboseBodyInsts(substs [][]ast.ASTRef, clause *Clause) (result error) {
	out, enabled, err := tabVerboseStream(1)
	if err != nil || !enabled {
		return err
	}
	m := s.manager
	vs, err := rewriter.NewVarSubstitution(m, false)
	if err != nil {
		return errors.Join(err, vs.Close())
	}
	var held []ast.ASTRef
	defer func() {
		for i := range held {
			result = errors.Join(result, m.DelRef(&held[i]))
		}
		result = errors.Join(result, vs.Close())
	}()
	hold := func(ref ast.ASTRef) ast.ASTHandle {
		held = append(held, ref)
		h, _ := m.RefGet(ref)
		return h
	}
	var subst []ast.ASTHandle
	for i := len(substs); i > 0; {
		i--
		sub := make([]ast.ASTHandle, len(substs[i]))
		for j := range substs[i] {
			if sub[j], err = m.RefGet(substs[i][j]); err != nil {
				return err
			}
		}
		for k := range subst {
			if subst[k].IsNull() {
				if k < len(sub) {
					subst[k] = sub[k]
				}
				continue
			}
			tmp, err := m.NewRef(ast.ASTHandle{})
			if err != nil {
				return err
			}
			if err = vs.Substitute(subst[k], sub, tmp); err != nil {
				return errors.Join(err, m.DelRef(&tmp))
			}
			subst[k] = hold(tmp)
		}
		for k := len(subst); k < len(sub); k++ {
			subst = append(subst, sub[k])
		}
	}
	bodyRef, err := clause.Body()
	if err != nil {
		return err
	}
	body := hold(bodyRef)
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	if err = vs.Substitute(body, subst, tmp); err != nil {
		return errors.Join(err, m.DelRef(&tmp))
	}
	if err = writeMkPP(out, m, hold(tmp)); err != nil {
		return err
	}
	_ = out.stream.AppendCString("\n")
	return out.stream.Flush()
}

// tabPPIndent is mk_pp(h, m, indent): later lines indented by indent.
func tabPPIndent(m *ast.ASTManager, h ast.ASTHandle, indent int) (string, error) {
	var b strings.Builder
	if err := writeMkPP(&b, m, h); err != nil {
		return "", err
	}
	return strings.ReplaceAll(b.String(), "\n", "\n"+strings.Repeat(" ", indent)), nil
}

func tabVerboseCheck(m *ast.ASTManager, predicates int, postcond ast.ASTHandle) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	out, enabled, err := tabVerboseStream(2)
	if err != nil || !enabled {
		return err
	}
	text, err := tabPPIndent(m, postcond, 7+predicates)
	if err != nil {
		return err
	}
	_ = out.stream.AppendCString(strings.Repeat(" ", predicates) + "check: " + text + "\n")
	return out.stream.Flush()
}

// tabVerboseNonGround is IF_VERBOSE(1, "TBD: non-ground\n" << mk_pp(postcond)
// << "\n"; m_clause->display(); "\n=>\n"; g.display(); "\n").
func tabVerboseNonGround(m *ast.ASTManager, postcond ast.ASTHandle, query, g *Clause) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	out, enabled, err := tabVerboseStream(1)
	if err != nil || !enabled {
		return err
	}
	_ = out.stream.AppendCString("TBD: non-ground\n")
	if err = writeMkPP(out, m, postcond); err != nil {
		return err
	}
	_ = out.stream.AppendCString("\n")
	if query != nil {
		if err = query.Display(out); err != nil {
			return err
		}
	}
	_ = out.stream.AppendCString("\n=>\n")
	if err = g.Display(out); err != nil {
		return err
	}
	_ = out.stream.AppendCString("\n")
	return out.stream.Flush()
}
