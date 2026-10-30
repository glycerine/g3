package spacer_test

// Source: src/muz/spacer/spacer_context.cpp pob and pob_queue.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/spacer"
)

func TestSpacerProofObligationBindingsDirtyStateAndQueue(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	truth, err := manager.MkTrue()
	spacerMust(t, err)
	falsehood, err := manager.MkFalse()
	spacerMust(t, err)
	transformer, err := spacer.NewPredicateTransformer(owner, "proof-obligation", falsehood, truth, spacer.PropSolverConfig{})
	spacerMust(t, err)
	x := spacerBool(t, manager, "proof-obligation-x")
	root, err := spacer.NewProofObligation(nil, transformer, 2, 0, x, []ast.ASTHandle{x})
	spacerMust(t, err)
	post, err := root.Post()
	spacerMust(t, err)
	skolem, err := root.Skolems()
	spacerMust(t, err)
	if len(skolem) != 1 || post != skolem[0] || post == x {
		t.Fatal("proof-obligation skolem replacement", post, skolem, x)
	}
	notPost, err := manager.MkNot(post)
	spacerMust(t, err)
	added, err := transformer.AddLemma(notPost, spacer.InfinityLevel)
	spacerMust(t, err)
	if !added {
		t.Fatal("blocking lemma not added")
	}
	blocked, usesLevel, err := transformer.IsBlocked(root)
	spacerMust(t, err)
	if !blocked || usesLevel != spacer.InfinityLevel {
		t.Fatal("ground proof-obligation blocking", blocked, usesLevel)
	}
	qblocked, err := transformer.IsQuantifierBlocked(root)
	spacerMust(t, err)
	if !qblocked {
		t.Fatal("quantifier proof-obligation blocking")
	}
	lemma, err := spacer.NewLemmaFromProofObligation(root, nil, 2)
	spacerMust(t, err)
	lemmaBody, err := lemma.Body()
	spacerMust(t, err)
	if kind, err := manager.GetKind(lemmaBody); err != nil || kind != ast.ASTQuantifier {
		t.Fatal("proof-obligation quantified lemma", kind, err)
	}
	instances, err := lemma.Instances()
	spacerMust(t, err)
	if len(instances) != 1 {
		t.Fatal("proof-obligation lemma instances", len(instances))
	}
	instance, err := manager.RefGet(instances[0])
	spacerMust(t, err)
	notX, err := manager.MkNot(x)
	spacerMust(t, err)
	if instance != notX {
		t.Fatal("proof-obligation lemma instance", instance, notX)
	}
	spacerMust(t, manager.DelRef(&instances[0]))
	spacerMust(t, lemma.Close())
	// When the unsat core no longer mentions an obligation skolem, the
	// source emits a ground clause and does not copy the obligation bindings.
	y := spacerBool(t, manager, "proof-obligation-independent-core")
	for _, literal := range []ast.ASTHandle{truth, y} {
		ground, err := spacer.NewLemmaFromProofObligation(root, []ast.ASTHandle{literal}, 2)
		spacerMust(t, err)
		body, err := ground.Body()
		spacerMust(t, err)
		expected, err := manager.PushNot(literal)
		spacerMust(t, err)
		if body != expected {
			t.Fatal("skolem-free core must produce a ground lemma without vacuous binders", body, expected)
		}
		instances, err := ground.Instances()
		spacerMust(t, err)
		if len(instances) != 0 {
			t.Fatal("skolem-free lemma copied obligation bindings", len(instances))
		}
		spacerMust(t, ground.Close())
	}
	spacerMust(t, root.NewPost(truth))
	if dirty, err := root.Dirty(); err != nil || !dirty {
		t.Fatal("dirty proof obligation", dirty, err)
	}
	spacerMust(t, root.Clean())
	if post, err = root.Post(); err != nil || post != truth {
		t.Fatal("clean proof obligation", post, truth, err)
	}
	child, err := spacer.NewProofObligation(root, transformer, 1, 1, truth, nil)
	spacerMust(t, err)
	deep, err := spacer.NewProofObligation(root, transformer, 2, 1, truth, nil)
	spacerMust(t, err)
	var queue spacer.ProofObligationQueue
	spacerMust(t, queue.SetRoot(root))
	spacerMust(t, queue.Push(deep))
	spacerMust(t, queue.Push(child))
	top, err := queue.Top()
	spacerMust(t, err)
	if top != child {
		t.Fatal("proof-obligation queue priority", top, child)
	}
	popped, err := queue.Pop()
	spacerMust(t, err)
	if popped != child {
		t.Fatal("proof-obligation queue pop", popped, child)
	}
	top, err = queue.Top()
	spacerMust(t, err)
	if top != root {
		t.Fatal("proof-obligation root eligibility", top, root)
	}
	spacerMust(t, root.Close())
	if closed, err := child.IsClosed(); err != nil || !closed {
		t.Fatal("proof-obligation close cascade", closed, err)
	}
	spacerMust(t, child.Release())
	spacerMust(t, deep.Release())
	spacerMust(t, root.Release())
	spacerMust(t, transformer.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}

func TestSpacerProofObligationQueueUsesPredicateTieBreaker(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	truth, err := manager.MkTrue()
	spacerMust(t, err)
	childApplication := spacerBool(t, manager, "queue-child-predicate")
	childDeclaration, err := manager.GetDecl(childApplication)
	spacerMust(t, err)
	rootApplication := spacerBool(t, manager, "queue-root-predicate")
	rootDeclaration, err := manager.GetDecl(rootApplication)
	spacerMust(t, err)
	childTransformer, err := spacer.NewRulePredicateTransformer(owner, childDeclaration, spacer.PropSolverConfig{})
	spacerMust(t, err)
	rootTransformer, err := spacer.NewRulePredicateTransformer(owner, rootDeclaration, spacer.PropSolverConfig{})
	spacerMust(t, err)
	root, err := rootTransformer.NewProofObligation(nil, 1, 1, truth, nil)
	spacerMust(t, err)
	child, err := childTransformer.NewProofObligation(root, 1, 1, truth, nil)
	spacerMust(t, err)
	var queue spacer.ProofObligationQueue
	spacerMust(t, queue.SetRoot(root))
	spacerMust(t, queue.Push(child))
	top, err := queue.Top()
	spacerMust(t, err)
	if top != child {
		t.Fatal("predicate declaration tie-breaker did not prioritize child")
	}
	spacerMust(t, root.Close())
	spacerMust(t, childTransformer.Close())
	spacerMust(t, rootTransformer.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}

func TestSpacerProofObligationStoreReusesNormalizedPostAndParent(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	truth, err := manager.MkTrue()
	spacerMust(t, err)
	falsehood, err := manager.MkFalse()
	spacerMust(t, err)
	transformer, err := spacer.NewPredicateTransformer(owner, "proof-obligation-store", falsehood, truth, spacer.PropSolverConfig{})
	spacerMust(t, err)
	p := spacerBool(t, manager, "proof-obligation-store-p")
	first, err := transformer.NewProofObligation(nil, 1, 2, p, nil)
	spacerMust(t, err)
	second, err := transformer.NewProofObligation(nil, 4, 5, p, nil)
	spacerMust(t, err)
	if first != second {
		t.Fatal("normalized proof obligation was not reused")
	}
	if level, err := first.Level(); err != nil || level != 4 {
		t.Fatal("reused proof-obligation level", level, err)
	}
	spacerMust(t, transformer.SetProofObligationReuse(false))
	third, err := transformer.NewProofObligation(nil, 4, 5, p, nil)
	spacerMust(t, err)
	if third == first {
		t.Fatal("disabled proof-obligation reuse")
	}
	spacerMust(t, transformer.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}
