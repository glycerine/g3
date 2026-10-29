package sls

// Source: src/tactic/sls/sls_evaluator.h:523-823.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (e *Evaluator) updateNodes(search *SearchTracker, starts []ast.ASTHandle, serious bool) (result error) {
	if err := e.open(); err != nil {
		return err
	}
	if search == nil || search.open() != nil || search.base != e.tracker {
		return ErrSLSSearch
	}
	var maxDepth uint32
	for _, node := range starts {
		depth, err := e.tracker.Distance(node)
		if err != nil {
			return err
		}
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	buckets := make([][]ast.ASTHandle, maxDepth+1)
	for _, node := range starts {
		depth, _ := e.tracker.Distance(node)
		buckets[depth] = append(buckets[depth], node)
	}
	visited := make(map[ast.ASTHandle]struct{})
	value := numeral.NewInt(0)
	defer func() { result = errors.Join(result, e.numbers.Del(&value)) }()
	for depth := int(maxDepth); depth >= 0; depth-- {
		for _, node := range buckets[depth] {
			if err := e.Evaluate(node, &value); err != nil {
				return err
			}
			if err := e.tracker.SetValue(node, &value); err != nil {
				return err
			}
			newScore, err := search.Score(node)
			if err != nil {
				return err
			}
			isTop, err := e.tracker.IsTopExpr(node)
			if err != nil {
				return err
			}
			oldScore, err := e.tracker.Score(node)
			if err != nil {
				return err
			}
			if isTop {
				if err = e.tracker.AdaptTopSum(node, newScore, oldScore); err != nil {
					return err
				}
			}
			if err = e.tracker.SetScore(node, newScore); err != nil {
				return err
			}
			if serious {
				if err = e.tracker.SetScorePrune(node, newScore); err != nil {
					return err
				}
			}
			links, found := e.tracker.uplinks[node]
			if !found {
				continue
			}
			for _, parent := range links {
				if _, found := visited[parent]; found {
					continue
				}
				parentDepth, err := e.tracker.Distance(parent)
				if err != nil || parentDepth >= uint32(depth) {
					if err != nil {
						return err
					}
					return ErrSLSSearch
				}
				buckets[parentDepth] = append(buckets[parentDepth], parent)
				visited[parent] = struct{}{}
			}
		}
	}
	return nil
}

func (e *Evaluator) UpdateAll(search *SearchTracker) error {
	starts := make([]ast.ASTHandle, 0, len(e.tracker.entryPoints))
	for _, entry := range e.tracker.entryPoints {
		starts = append(starts, entry)
	}
	return e.updateNodes(search, starts, true)
}

func (e *Evaluator) UpdateConstant(search *SearchTracker, decl ast.ASTHandle, value *numeral.Int) error {
	if err := e.tracker.SetDeclValue(decl, value); err != nil {
		return err
	}
	entry, err := e.tracker.EntryPoint(decl)
	if err != nil {
		return err
	}
	return e.updateNodes(search, []ast.ASTHandle{entry}, false)
}

func (e *Evaluator) SeriousUpdate(search *SearchTracker, decl ast.ASTHandle, value *numeral.Int) error {
	if err := e.tracker.SetDeclValue(decl, value); err != nil {
		return err
	}
	entry, err := e.tracker.EntryPoint(decl)
	if err != nil {
		return err
	}
	return e.updateNodes(search, []ast.ASTHandle{entry}, true)
}

// UpdatePrune preserves the source's Boolean occurrence criterion. The value
// propagation and score publication are identical to UpdateConstant; the
// returned flag says whether a changed Boolean score can benefit a retained
// positive or negative occurrence.
func (e *Evaluator) UpdatePrune(search *SearchTracker, decl ast.ASTHandle, value *numeral.Int) (bool, error) {
	before := make(map[ast.ASTHandle]float64)
	for node, score := range e.tracker.scores {
		if score.hasPosOcc != 0 || score.hasNegOcc != 0 {
			before[node] = score.score
		}
	}
	if err := e.UpdateConstant(search, decl, value); err != nil {
		return false, err
	}
	for node, old := range before {
		score := e.tracker.scores[node]
		baseline := score.scorePrune
		// The first prune pass compares against the last serious score. Preserve
		// old only as an assertion that this node was actually reached.
		if score.score != old {
			if score.score > baseline && score.hasPosOcc != 0 {
				return true, nil
			}
			if score.score <= baseline && score.hasNegOcc != 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

func (e *Evaluator) RandomizeLocal(search *SearchTracker, declarations []ast.ASTHandle) (result error) {
	if len(declarations) == 0 {
		return ErrSLSSearch
	}
	random, err := search.RandomUint(16)
	if err != nil {
		return err
	}
	decl := declarations[random%uint32(len(declarations))]
	sort, err := e.manager.GetRange(decl)
	if err != nil {
		return err
	}
	value := numeral.NewInt(0)
	defer func() { result = errors.Join(result, e.numbers.Del(&value)) }()
	if err = search.RandomValue(sort, &value); err != nil {
		return err
	}
	return e.SeriousUpdate(search, decl, &value)
}
