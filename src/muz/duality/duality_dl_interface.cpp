/*++
  Copyright (c) 2013 Microsoft Corporation

  Module Name:

  duality_dl_interface.cpp

  Abstract:

  SMT2 interface for Duality

  Author:

  Krystof Hoder (t-khoder) 2011-9-22.
  Modified by Ken McMIllan (kenmcmil) 2013-4-18.

  Revision History:

  --*/

#include "muz/base/dl_context.h"
#include "muz/transforms/dl_mk_coi_filter.h"
#include "muz/transforms/dl_mk_interp_tail_simplifier.h"
#include "muz/transforms/dl_mk_subsumption_checker.h"
#include "muz/transforms/dl_mk_rule_inliner.h"
#include "muz/base/dl_rule.h"
#include "muz/base/dl_rule_transformer.h"
#include "parsers/smt2/smt2parser.h"
#include "muz/duality/duality_dl_interface.h"
#include "muz/base/dl_rule_set.h"
#include "muz/transforms/dl_mk_slice.h"
#include "muz/transforms/dl_mk_unfold.h"
#include "muz/transforms/dl_mk_coalesce.h"
#include "ast/expr_abstract.h"
#include "model/model_smt2_pp.h"
#include "model/model_v2_pp.h"
#include "muz/base/fixedpoint_params.hpp"
#include "ast/used_vars.h"
#include "ast/func_decl_dependencies.h"
#include "muz/transforms/dl_transforms.h"
#include "ast/rewriter/var_subst.h"
#include "ast/rewriter/expr_safe_replace.h"
#include "model/model_evaluator.h"
#include "smt/smt_solver.h"
#include "ast/ast_util.h"

// template class symbol_table<family_id>;

#ifdef WIN32
#pragma warning(disable:4996)
#pragma warning(disable:4800)
#pragma warning(disable:4267)
#pragma warning(disable:4101)
#endif

#include "duality/duality.h"
#include "duality/duality_profiling.h"

// using namespace Duality;

namespace Duality {

    enum DualityStatus {StatusModel, StatusRefutation, StatusUnknown, StatusNull};

    class duality_data {
    public:
        context ctx;
        RPFP::LogicSolver *ls;
        RPFP *rpfp;

        DualityStatus status;
        std::vector<expr> clauses;
        std::vector<expr> axioms;   // the background axioms, for certificate witnesses
        std::vector<std::vector<RPFP::label_struct> > clause_labels;
        hash_map<RPFP::Edge *,int> map;  // edges to clauses
        Solver *old_rs;
        Solver::Counterexample cex;

        duality_data(ast_manager &_m) : ctx(_m,config(params_ref())) {
            ls = nullptr;
            rpfp = nullptr;
            status = StatusNull;
            old_rs = nullptr;
        }
        ~duality_data(){
            if(old_rs)
                dealloc(old_rs);
            if(rpfp)
                dealloc(rpfp);
            if(ls)
                dealloc(ls);
        }
    };


    dl_interface::dl_interface(datalog::context& dl_ctx) :
        engine_base(dl_ctx.get_manager(), "duality"),
        m_ctx(dl_ctx)

    {
        _d = nullptr;
        //   dl_ctx.get_manager().toggle_proof_mode(PGM_FINE);
    }


    dl_interface::~dl_interface() {
        if(_d)
            dealloc(_d);
    }


    //
    // Check if the new rules are weaker so that we can
    // re-use existing context.
    //
#if 0
    void dl_interface::check_reset() {
        // TODO
        datalog::rule_ref_vector const& new_rules = m_ctx.get_rules().get_rules();
        datalog::rule_ref_vector const& old_rules = m_old_rules.get_rules();
        bool is_subsumed = !old_rules.empty();
        for (unsigned i = 0; is_subsumed && i < new_rules.size(); ++i) {
            is_subsumed = false;
            for (unsigned j = 0; !is_subsumed && j < old_rules.size(); ++j) {
                if (m_ctx.check_subsumes(*old_rules[j], *new_rules[i])) {
                    is_subsumed = true;
                }
            }
            if (!is_subsumed) {
                TRACE("pdr", new_rules[i]->display(m_ctx, tout << "Fresh rule "););
                m_context->reset();
            }
        }
        m_old_rules.reset();
        m_old_rules.add_rules(new_rules.size(), new_rules.c_ptr());
    }
#endif


    // FORK FIX #99 (g3 audit, see FORK_BUGFIXES.md): validate a counterexample that relies on quantifiers (AUDIT duality_itp_sat)
    //
    // Duality's solvers use MBQI only for interpolant quantifiers
    // (mbqi.id=itp), so an SMT sat on a formula with other quantifiers is
    // trusted although they were never checked: (=> (and (= x 1) (forall
    // ((z Int)) (> z x))) err) was answered sat. When a step of the
    // counterexample or a background axiom has a quantifier, check the
    // derivation against the original rules with a full solver: each step's
    // rule, instantiated with fresh constants, must derive its fact from the
    // facts of the steps before it (child-first order), every predicate
    // atom of its body standing for the disjunction of those facts. A
    // derivation that fails, or that has a predicate in a non-positive
    // position, is reported as incompleteness.

    // Child-first list of the steps used by a counterexample, as print_proof
    // visits them.
    static bool cex_steps(RPFP *tree, RPFP::Node *node, std::vector<RPFP::Node *> &steps) {
        RPFP::Edge *edge = node->Outgoing;
        if (!edge)
            return false;
        for (unsigned i = 0; i < edge->Children.size(); i++)
            if (!tree->Empty(edge->Children[i]))
                if (!cex_steps(tree, edge->Children[i], steps))
                    return false;
        steps.push_back(node);
        return true;
    }

    static bool cex_contains_pred(obj_hashtable< ::func_decl> const &preds, ::expr *e, ast_mark &visited) {
        if (visited.is_marked(e))
            return false;
        visited.mark(e, true);
        if (is_app(e)) {
            if (preds.contains(to_app(e)->get_decl()))
                return true;
            for (unsigned i = 0; i < to_app(e)->get_num_args(); i++)
                if (cex_contains_pred(preds, to_app(e)->get_arg(i), visited))
                    return true;
            return false;
        }
        if (is_quantifier(e))
            return cex_contains_pred(preds, to_quantifier(e)->get_expr(), visited);
        return false;
    }

    // Replace each predicate atom in a positive position of a body by the
    // disjunction of the derived facts of its predicate. Returns false if a
    // predicate occurs in another position.
    static bool cex_replace_atoms(ast_manager &m, obj_hashtable< ::func_decl> const &preds, expr_ref_vector const &facts,
                                  ::expr *e, expr_ref &res) {
        if (is_app(e)) {
            app *a = to_app(e);
            if (preds.contains(a->get_decl())) {
                expr_ref_vector disjuncts(m);
                for (unsigned i = 0; i < facts.size(); i++) {
                    app *fact = to_app(facts.get(i));
                    if (fact->get_decl() != a->get_decl())
                        continue;
                    expr_ref_vector eqs(m);
                    for (unsigned j = 0; j < a->get_num_args(); j++)
                        eqs.push_back(m.mk_eq(a->get_arg(j), fact->get_arg(j)));
                    disjuncts.push_back(mk_and(m, eqs.size(), eqs.c_ptr()));
                }
                res = mk_or(m, disjuncts.size(), disjuncts.c_ptr());
                return true;
            }
            if (m.is_and(a) || m.is_or(a) || m.is_implies(a)) {
                expr_ref_vector args(m);
                for (unsigned i = 0; i < a->get_num_args(); i++) {
                    expr_ref arg(m);
                    if (m.is_implies(a) && i == 0) {
                        ast_mark visited;
                        if (cex_contains_pred(preds, a->get_arg(i), visited))
                            return false;
                        arg = a->get_arg(i);
                    }
                    else if (!cex_replace_atoms(m, preds, facts, a->get_arg(i), arg))
                        return false;
                    args.push_back(arg);
                }
                res = m.mk_app(a->get_decl(), args.size(), args.c_ptr());
                return true;
            }
            ast_mark visited;
            if (cex_contains_pred(preds, e, visited))
                return false;
            res = e;
            return true;
        }
        if (is_quantifier(e)) {
            expr_ref body(m);
            if (!cex_replace_atoms(m, preds, facts, to_quantifier(e)->get_expr(), body))
                return false;
            res = m.update_quantifier(to_quantifier(e), body);
            return true;
        }
        res = e;
        return true;
    }

    static bool validate_counterexample(dl_interface *d, RPFP *tree, RPFP::Node *root) {
        duality_data *dd = d->dd();
        ast_manager &m = dd->ctx.m();
        std::vector<RPFP::Node *> steps;
        if (!cex_steps(tree, root, steps))
            return false;
        // only a step rule or an axiom with a quantifier needs the check
        bool quantified = false;
        for (unsigned i = 0; i < dd->axioms.size(); i++)
            if (has_quantifiers(to_expr(dd->axioms[i].raw())))
                quantified = true;
        for (unsigned i = 0; i < steps.size(); i++) {
            ::expr *t = to_expr(dd->clauses[dd->map[steps[i]->Outgoing->map]].raw());
            if (is_quantifier(t) && to_quantifier(t)->is_forall())
                t = to_quantifier(t)->get_expr();
            if (has_quantifiers(t))
                quantified = true;
        }
        if (!quantified)
            return true;
        // the predicate symbols are the uninterpreted clause heads
        obj_hashtable< ::func_decl> preds;
        for (unsigned i = 0; i < dd->clauses.size(); i++) {
            ::expr *c = to_expr(dd->clauses[i].raw());
            if (is_quantifier(c))
                c = to_quantifier(c)->get_expr();
            if (m.is_implies(c))
                c = to_app(c)->get_arg(1);
            if (is_app(c) && to_app(c)->get_family_id() == null_family_id)
                preds.insert(to_app(c)->get_decl());
        }
        expr_ref_vector facts(m), constraints(m);
        for (unsigned i = 0; i < steps.size(); i++) {
            ::expr *t = to_expr(dd->clauses[dd->map[steps[i]->Outgoing->map]].raw());
            expr_ref inst(t, m);
            if (is_quantifier(t) && to_quantifier(t)->is_forall()) {
                quantifier *q = to_quantifier(t);
                unsigned n = q->get_num_decls();
                expr_ref_vector skolem(m);
                skolem.resize(n);
                for (unsigned j = 0; j < n; j++)
                    skolem[n - 1 - j] = m.mk_fresh_const("@cex", q->get_decl_sort(j));
                var_subst vs(m, false);
                vs(q->get_expr(), n, skolem.c_ptr(), inst);
            }
            // the clause shape FromClauses gives it
            expr_ref body(m), head(m);
            if (is_app(inst) && to_app(inst)->get_family_id() == null_family_id) {
                body = m.mk_true();
                head = inst;
            }
            else if (m.is_implies(inst) && (m.is_false(to_app(inst)->get_arg(1)) ||
                                             (is_app(to_app(inst)->get_arg(1)) &&
                                              to_app(to_app(inst)->get_arg(1))->get_family_id() == null_family_id))) {
                body = to_app(inst)->get_arg(0);
                head = to_app(inst)->get_arg(1);
            }
            else {
                body = m.mk_not(inst);
                head = m.mk_false();
            }
            // the query step is the root; every other step derives a fact
            if (m.is_false(head) != (i + 1 == steps.size()))
                return false;
            expr_ref condition(m);
            if (!cex_replace_atoms(m, preds, facts, body, condition))
                return false;
            constraints.push_back(condition);
            if (!m.is_false(head)) {
                ::func_decl *f = to_app(head)->get_decl();
                expr_ref_vector args(m);
                for (unsigned j = 0; j < f->get_arity(); j++) {
                    args.push_back(m.mk_fresh_const("@cex", f->get_domain(j)));
                    constraints.push_back(m.mk_eq(to_app(head)->get_arg(j), args.get(j)));
                }
                facts.push_back(m.mk_app(f, args.size(), args.c_ptr()));
            }
        }
        // query runs with proofs on; MBQI with proofs can loop on a valid
        // forall-exists body, so check without them
        scoped_no_proof _np(m);
        ref< ::solver> s = mk_smt_solver(m, params_ref(), ::symbol::null);
        for (unsigned i = 0; i < dd->axioms.size(); i++)
            s->assert_expr(to_expr(dd->axioms[i].raw()));
        for (unsigned i = 0; i < constraints.size(); i++)
            s->assert_expr(constraints.get(i));
        return s->check_sat(0, nullptr) == l_true;
    }

    lbool dl_interface::query(::expr * query) {

        // we restore the initial state in the datalog context
        m_ctx.ensure_opened();

        // if there is old data, get the cex and dispose (later)
        duality_data *old_data = _d;
        Solver *old_rs = nullptr;
        if(old_data){
            old_rs = old_data->old_rs;
            // FORK FIX #44 (g3 audit, see FORK_BUGFIXES.md): a query that threw left no old solver (AUDIT duality_01-F3 follow-up)
            if(old_rs)
                old_rs->GetCounterexample().swap(old_data->cex);
        }

        scoped_proof generate_proofs_please(m_ctx.get_manager());

        // make a new problem and solver
        _d = alloc(duality_data,m_ctx.get_manager());
        _d->ctx.set("mbqi",m_ctx.get_params().duality_mbqi());
        _d->ls = alloc(RPFP::iZ3LogicSolver,_d->ctx);
        _d->rpfp = alloc(RPFP,_d->ls);



        expr_ref_vector rules(m_ctx.get_manager());
        svector< ::symbol> names;
        unsigned_vector bounds;
        // m_ctx.get_rules_as_formulas(rules, names);


        // If using SAS 2013 abstractiion, we need to perform some transforms
        expr_ref query_ref(m_ctx.get_manager());
        if(m_ctx.quantify_arrays()){
            datalog::rule_manager& rm = m_ctx.get_rule_manager();
            // FORK FIX #43 (g3 audit, see FORK_BUGFIXES.md): restore the untransformed rules after the query (AUDIT muz_fp_03)
            datalog::rule_set old_rules(m_ctx.get_rules());
            rm.mk_query(query, m_ctx.get_rules());
            apply_default_transformation(m_ctx);
            datalog::rule_set &rs = m_ctx.get_rules();
            if(m_ctx.get_rules().get_output_predicates().empty())
                query_ref = m_ctx.get_manager().mk_false();
            else {
                func_decl_ref query_pred(m_ctx.get_manager());
                query_pred = m_ctx.get_rules().get_output_predicate();
                ptr_vector<sort> sorts;
                unsigned nargs = query_pred.get()->get_arity();
                expr_ref_vector vars(m_ctx.get_manager());
                for(unsigned i = 0; i < nargs; i++){
                    ::sort *s = query_pred.get()->get_domain(i);
                    vars.push_back(m_ctx.get_manager().mk_var(nargs-1-i,s));
                }
                query_ref = m_ctx.get_manager().mk_app(query_pred.get(),nargs,vars.c_ptr());
                query = query_ref.get();
            }
            unsigned nrules = rs.get_num_rules();
            for(unsigned i = 0; i < nrules; i++){
                expr_ref f(m_ctx.get_manager());
                rm.to_formula(*rs.get_rule(i), f);
                rules.push_back(f);
            }
            m_ctx.reopen();
            m_ctx.replace_rules(old_rules);
        }
        else
            m_ctx.get_raw_rule_formulas(rules, names, bounds);

        // get all the rules as clauses
        std::vector<expr> &clauses = _d->clauses;
        clauses.clear();
        for (unsigned i = 0; i < rules.size(); ++i) {
            expr e(_d->ctx,rules[i].get());
            clauses.push_back(e);
        }

        std::vector<sort> b_sorts;
        std::vector<symbol> b_names;
        used_vars uv;
        uv.process(query);
        unsigned nuv = uv.get_max_found_var_idx_plus_1();
        for(int i = nuv-1; i >= 0; i--){ // var indices are backward
            ::sort * s = uv.get(i);
            if(!s)
                s = _d->ctx.m().mk_bool_sort(); // missing var, whatever
            b_sorts.push_back(sort(_d->ctx,s));
            b_names.push_back(symbol(_d->ctx,::symbol(i))); // names?
        }

#if 0
        // turn the query into a clause
        expr q(_d->ctx,m_ctx.bind_variables(query,false));

        std::vector<sort> b_sorts;
        std::vector<symbol> b_names;
        if (q.is_quantifier() && !q.is_quantifier_forall()) {
            int bound = q.get_quantifier_num_bound();
            for(int j = 0; j < bound; j++){
                b_sorts.push_back(q.get_quantifier_bound_sort(j));
                b_names.push_back(q.get_quantifier_bound_name(j));
            }
            q = q.arg(0);
        }
#else
        expr q(_d->ctx,query);
#endif

        expr qc = implies(q,_d->ctx.bool_val(false));
        qc = _d->ctx.make_quant(Forall,b_sorts,b_names,qc);
        clauses.push_back(qc);
        bounds.push_back(UINT_MAX);

        // get the background axioms
        unsigned num_asserts = m_ctx.get_num_assertions();
        for (unsigned i = 0; i < num_asserts; ++i) {
            expr e(_d->ctx,m_ctx.get_assertion(i));
            _d->rpfp->AssertAxiom(e);
            // FORK FIX #63 (g3 audit, see FORK_BUGFIXES.md): keep the raw axioms for certificate witnesses (AUDIT duality_01-F4)
            _d->axioms.push_back(e);
        }

        // make sure each predicate is the head of at least one clause
        func_decl_set heads;
        for(unsigned i = 0; i < clauses.size(); i++){
            expr cl = clauses[i];

            while(true){
                if(cl.is_app()){
                    decl_kind k = cl.decl().get_decl_kind();
                    if(k == Implies)
                        cl = cl.arg(1);
                    else {
                        heads.insert(cl.decl());
                        break;
                    }
                }
                else if(cl.is_quantifier())
                    cl = cl.body();
                else break;
            }
        }
        ast_ref_vector const &pinned = m_ctx.get_pinned();
        for(unsigned i = 0; i < pinned.size(); i++){
            ::ast *fa = pinned[i];
            if(is_func_decl(fa)){
                ::func_decl *fd = to_func_decl(fa);
                if (m_ctx.is_predicate(fd)) {
                    func_decl f(_d->ctx, fd);
                    if (!heads.contains(fd)) {
                        int arity = f.arity();
                        std::vector<expr> args;
                        args.reserve(arity);
                        for (int j = 0; j < arity; j++)
                            args.push_back(_d->ctx.fresh_func_decl("X", f.domain(j))());
                        expr c = implies(_d->ctx.bool_val(false), f(args));
                        c = _d->ctx.make_quant(Forall, args, c);
                        clauses.push_back(c);
                        bounds.push_back(UINT_MAX);
                    }
                }
            }
        }
        unsigned rb = m_ctx.get_params().duality_recursion_bound();
        std::vector<unsigned> std_bounds;
        for(unsigned i = 0; i < bounds.size(); i++){
            unsigned b = bounds[i];
            if (b == UINT_MAX) b = rb;
            std_bounds.push_back(b);
        }

        // creates 1-1 map between clauses and rpfp edges
        _d->rpfp->FromClauses(clauses,&std_bounds);

        // populate the edge-to-clause map
        // FORK FIX #91 (g3 audit, see FORK_BUGFIXES.md): map each edge to its own clause after RemoveDeadNodes (AUDIT duality zbug round 2)
        // FromClauses creates edge number i+1 for clause i, but RemoveDeadNodes
        // may delete edges, so the position in edges is not the clause index.
        for(unsigned i = 0; i < _d->rpfp->edges.size(); ++i)
            _d->map[_d->rpfp->edges[i]] = _d->rpfp->edges[i]->number - 1;

        // create a solver object

        Solver *rs = Solver::Create("duality", _d->rpfp);

        if(old_rs)
            rs->LearnFrom(old_rs); // new solver gets hints from old solver

        // set its options
        IF_VERBOSE(1, rs->SetOption("report","1"););
        rs->SetOption("full_expand",m_ctx.get_params().duality_full_expand() ? "1" : "0");
        rs->SetOption("no_conj",m_ctx.get_params().duality_no_conj() ? "1" : "0");
        rs->SetOption("feasible_edges",m_ctx.get_params().duality_feasible_edges() ? "1" : "0");
        rs->SetOption("use_underapprox",m_ctx.get_params().duality_use_underapprox() ? "1" : "0");
        rs->SetOption("stratified_inlining",m_ctx.get_params().duality_stratified_inlining() ? "1" : "0");
        rs->SetOption("batch_expand",m_ctx.get_params().duality_batch_expand() ? "1" : "0");
        rs->SetOption("conjecture_file",m_ctx.get_params().duality_conjecture_file());
        rs->SetOption("enable_restarts",m_ctx.get_params().duality_enable_restarts() ? "1" : "0");
#if 0
        if(rb != UINT_MAX){
            std::ostringstream os; os << rb;
            rs->SetOption("recursion_bound", os.str());
        }
#endif

        // Solve!
        bool ans;
        try {
            ans = rs->Solve();
            // FORK FIX #99 (g3 audit, see FORK_BUGFIXES.md): a counterexample that relies on quantifiers must pass validate_counterexample (AUDIT duality_itp_sat)
            if(!ans && !validate_counterexample(this, rs->GetCounterexample().get_tree(), rs->GetCounterexample().get_root()))
                throw Duality::Solver::Incompleteness();
        }
        catch (Duality::solver::cancel_exception &exn){
            throw default_exception(Z3_CANCELED_MSG);
        }
        catch (Duality::Solver::Incompleteness &exn){
            throw default_exception("incompleteness");
        }

        // profile!

        if(m_ctx.get_params().duality_profile())
            print_profile(std::cout);

        // save the result and counterexample if there is one
        _d->status = ans ? StatusModel : StatusRefutation;
        _d->cex.swap(rs->GetCounterexample()); // take ownership of cex
        _d->old_rs = rs; // save this for later hints

        if(old_data){
            dealloc(old_data); // this deallocates the old solver if there is one
        }

        // dealloc(rs); this is now owned by data

        // true means the RPFP problem is SAT, so the query is UNSAT
        // but we return undef if the UNSAT result is bounded
        if(ans){
            if(rs->IsResultRecursionBounded()){
#if 0
                m_ctx.set_status(datalog::BOUNDED);
                return l_undef;
#else
                return l_false;
#endif
            }
            return l_false;
        }
        return l_true;
    }

    expr_ref dl_interface::get_cover_delta(int level, ::func_decl* pred_orig) {
        SASSERT(false);
        return expr_ref(m_ctx.get_manager());
    }

    void dl_interface::add_cover(int level, ::func_decl* pred, ::expr* property) {
        SASSERT(false);
    }

    unsigned dl_interface::get_num_levels(::func_decl* pred) {
        SASSERT(false);
        return 0;
    }

    void dl_interface::collect_statistics(::statistics& st) const {
    }

    void dl_interface::reset_statistics() {
    }

    static hash_set<func_decl> *local_func_decls;

    // FORK FIX #63 (g3 audit, see FORK_BUGFIXES.md): certificate witnesses for rule variables qe-lite eliminated (AUDIT duality_01-F4)
    //
    // print_proof and extract_proof used tree->Eval(&edge, skolem) as the
    // value of each rule variable. A variable that qe-lite eliminated from
    // the edge formula is unconstrained in the counterexample model, so its
    // model default was printed: q0(5) was certified by x1 = x + 3 with x = 0.
    // As g3's recoverProofWitness does, solve the original rule for its
    // variables instead: the head must equal the step's proved fact, each
    // used body predicate must equal its child's proved fact, and the rest of
    // the body must hold together with the background axioms.

    // Split a clause (quantifier body) into its head and its body conjuncts.
    static ::expr *witness_clause_shape(ast_manager &m, ::expr *clause, ::expr *&body, ptr_vector< ::expr> &atoms) {
        atoms.reset();
        body = nullptr;
        if (!m.is_implies(clause))
            return clause;
        body = to_app(clause)->get_arg(0);
        if (m.is_and(body))
            atoms.append(to_app(body)->get_num_args(), to_app(body)->get_args());
        else
            atoms.push_back(body);
        return to_app(clause)->get_arg(1);
    }

    // The fact a counterexample node proves: its original rule's head symbol
    // applied to the evaluated edge parameters.
    static bool witness_node_fact(dl_interface *d, RPFP *tree, RPFP::Node *node, expr_ref &fact) {
        ast_manager &m = d->dd()->ctx.m();
        RPFP::Edge &edge = *node->Outgoing;
        ::expr *t = to_expr(d->dd()->clauses[d->dd()->map[edge.map]].raw());
        if (is_quantifier(t) && to_quantifier(t)->is_forall())
            t = to_quantifier(t)->get_expr();
        ::expr *body;
        ptr_vector< ::expr> atoms;
        ::expr *head = witness_clause_shape(m, t, body, atoms);
        if (!is_app(head))
            return false;
        ::func_decl *f = to_app(head)->get_decl();
        if (f->get_arity() > edge.F.IndParams.size())
            return false;
        std::vector<expr> vals;
        ptr_vector< ::expr> args;
        for (unsigned i = 0; i < f->get_arity(); i++) {
            vals.push_back(tree->Eval(&edge, edge.F.IndParams[i]));
            args.push_back(to_expr(vals.back().raw()));
        }
        fact = m.mk_app(f, args.size(), args.c_ptr());
        return true;
    }

    // Body positions (1-based) of each distinct predicate atom of a clause
    // (the quantifier body), one group per edge child, as g3's
    // proofClauseShape computes them. Returns false on a malformed program.
    static bool witness_positions(dl_interface *d, ::expr *clause, vector<unsigned_vector> &positions) {
        ast_manager &m = d->dd()->ctx.m();
        // the predicate symbols are the uninterpreted clause heads
        obj_hashtable< ::func_decl> preds;
        for (unsigned i = 0; i < d->dd()->clauses.size(); i++) {
            ::expr *c = to_expr(d->dd()->clauses[i].raw());
            if (is_quantifier(c))
                c = to_quantifier(c)->get_expr();
            if (m.is_implies(c))
                c = to_app(c)->get_arg(1);
            if (!is_app(c))
                return false;
            if (to_app(c)->get_family_id() == null_family_id)
                preds.insert(to_app(c)->get_decl());
        }
        ::expr *body;
        ptr_vector< ::expr> atoms;
        witness_clause_shape(m, clause, body, atoms);
        positions.reset();
        obj_map< ::expr, unsigned> seen;
        for (unsigned i = 0; i < atoms.size(); i++) {
            ::expr *a = atoms[i];
            if (!is_app(a) || !preds.contains(to_app(a)->get_decl()))
                continue;
            unsigned idx;
            if (!seen.find(a, idx)) {
                idx = positions.size();
                seen.insert(a, idx);
                positions.push_back(unsigned_vector());
            }
            positions[idx].push_back(i + 1);
        }
        return true;
    }

    // Values of the original rule's bound variables for this step, indexed
    // by declaration. Returns false if none are found; the caller then keeps
    // the counterexample model's values.
    static bool find_step_witness(dl_interface *d, RPFP *tree, RPFP::Node *root, expr_ref_vector &witness) {
        ast_manager &m = d->dd()->ctx.m();
        scoped_proof _sp(m);
        RPFP::Edge &edge = *root->Outgoing;
        ::expr *t = to_expr(d->dd()->clauses[d->dd()->map[edge.map]].raw());
        if (!is_quantifier(t) || !to_quantifier(t)->is_forall())
            return false;
        ::quantifier *q = to_quantifier(t);
        ::expr *clause = q->get_expr();
        try {
            // body positions of each distinct predicate atom, one group per edge child
            vector<unsigned_vector> positions;
            if (!witness_positions(d, clause, positions))
                return false;
            if (positions.size() != edge.Children.size())
                return false;
            // instantiate the rule with fresh constants
            unsigned n = q->get_num_decls();
            expr_ref_vector skolem(m);
            skolem.resize(n);
            for (unsigned j = 0; j < n; j++)
                skolem[n - 1 - j] = m.mk_fresh_const("@proof", q->get_decl_sort(j));
            var_subst vs(m, false);
            expr_ref inst(m);
            vs(clause, n, skolem.c_ptr(), inst);
            ::expr *ibody;
            ptr_vector< ::expr> iatoms;
            ::expr *ihead = witness_clause_shape(m, inst, ibody, iatoms);
            expr_ref ibody_ref(ibody ? ibody : m.mk_true(), m);
            if (!ibody)
                iatoms.push_back(ibody_ref.get());
            // predicate atoms of unused children are false, used ones are
            // true and their arguments equal the child's proved fact
            obj_map< ::expr, ::expr*> replacements;
            for (unsigned i = 0; i < positions.size(); i++)
                for (unsigned k = 0; k < positions[i].size(); k++) {
                    if (positions[i][k] > iatoms.size())
                        return false;
                    replacements.insert(iatoms[positions[i][k] - 1], m.mk_false());
                }
            expr_ref_vector constraints(m), facts(m);
            for (unsigned i = 0; i < edge.Children.size(); i++) {
                if (tree->Empty(edge.Children[i]))
                    continue;
                expr_ref fact(m);
                if (!witness_node_fact(d, tree, edge.Children[i], fact))
                    return false;
                facts.push_back(fact);
                for (unsigned k = 0; k < positions[i].size(); k++) {
                    ::expr *a = iatoms[positions[i][k] - 1];
                    if (!is_app(a) || to_app(a)->get_decl() != to_app(fact)->get_decl())
                        return false;
                    for (unsigned j = 0; j < to_app(a)->get_num_args(); j++)
                        constraints.push_back(m.mk_eq(to_app(a)->get_arg(j), to_app(fact)->get_arg(j)));
                    replacements.insert(a, m.mk_true());
                }
            }
            expr_ref conclusion(m);
            if (!witness_node_fact(d, tree, root, conclusion))
                return false;
            if (!is_app(ihead) || to_app(ihead)->get_decl() != to_app(conclusion)->get_decl())
                return false;
            for (unsigned j = 0; j < to_app(ihead)->get_num_args(); j++)
                constraints.push_back(m.mk_eq(to_app(ihead)->get_arg(j), to_app(conclusion)->get_arg(j)));
            expr_safe_replace rep(m);
            for (auto const& kv : replacements)
                rep.insert(kv.m_key, kv.m_value);
            expr_ref condition(m);
            rep(ibody_ref.get(), condition);
            constraints.push_back(condition);
            // solve in a fresh solver, with the background axioms
            ref< ::solver> s = mk_smt_solver(m, params_ref(), ::symbol::null);
            s->set_produce_models(true);
            for (unsigned i = 0; i < d->dd()->axioms.size(); i++)
                s->assert_expr(to_expr(d->dd()->axioms[i].raw()));
            for (unsigned i = 0; i < constraints.size(); i++)
                s->assert_expr(constraints.get(i));
            if (s->check_sat(0, nullptr) != l_true)
                return false;
            model_ref mdl;
            s->get_model(mdl);
            if (!mdl)
                return false;
            model_evaluator ev(*mdl.get());
            ev.set_model_completion(true);
            witness.reset();
            for (unsigned j = 0; j < n; j++)
                witness.push_back(ev(skolem.get(n - 1 - j)));
            return true;
        }
        catch (z3_exception &) {
            return false;
        }
    }

    static void print_proof(dl_interface *d, std::ostream& out, RPFP *tree, RPFP::Node *root) {
        context &ctx = d->dd()->ctx;
        RPFP::Node &node = *root;
        RPFP::Edge &edge = *node.Outgoing;

        // first, prove the children (that are actually used)

        for(unsigned i = 0; i < edge.Children.size(); i++){
            if(!tree->Empty(edge.Children[i])){
                print_proof(d,out,tree,edge.Children[i]);
            }
        }

        // print the label and the proved fact

        out << "(step s!" << node.number;
        out << " (" << node.Name.name();
        for(unsigned i = 0; i < edge.F.IndParams.size(); i++)
            out << " " << tree->Eval(&edge,edge.F.IndParams[i]);
        out << ")\n";

        // print the rule number

        out << " rule!" << node.Outgoing->map->number;

        // print the substitution

        out << "  (subst\n";
        RPFP::Edge *orig_edge = edge.map;
        int orig_clause = d->dd()->map[orig_edge];
        expr &t = d->dd()->clauses[orig_clause];
        if (t.is_quantifier() && t.is_quantifier_forall()) {
            int bound = t.get_quantifier_num_bound();
            std::vector<sort> sorts;
            std::vector<symbol> names;
            hash_map<int,expr> subst;
            // FORK FIX #63 (g3 audit, see FORK_BUGFIXES.md): print the original rule's witness (AUDIT duality_01-F4)
            expr_ref_vector witness(ctx.m());
            bool has_witness = find_step_witness(d,tree,root,witness);
            for(int j = 0; j < bound; j++){
                sort the_sort = t.get_quantifier_bound_sort(j);
                symbol name = t.get_quantifier_bound_name(j);
                expr skolem = ctx.constant(symbol(ctx,name),sort(ctx,the_sort));
                expr val = has_witness ? ctx.cook(witness.get(j)) : tree->Eval(&edge,skolem);
                out << "    (= " << skolem << " " << val << ")\n";
                expr local_skolem = tree->Localize(&edge,skolem);
                (*local_func_decls).insert(local_skolem.decl());
            }
        }
        out << "  )\n";

        out << "  (labels";
        std::vector<symbol> labels;
        tree->GetLabels(&edge,labels);
        for(unsigned j = 0; j < labels.size(); j++){
            out << " " << labels[j];
        }

        out << "  )\n";

        // reference the proofs of all the children, in syntactic order
        // "true" means the child is not needed

        out << "  (ref ";
        for(unsigned i = 0; i < edge.Children.size(); i++){
            if(!tree->Empty(edge.Children[i]))
                out << " s!" << edge.Children[i]->number;
            else
                out << " true";
        }
        out << "  )";
        out << ")\n";
    }


    void dl_interface::display_certificate(std::ostream& out) const {
        ((dl_interface *)this)->display_certificate_non_const(out);
    }

    void dl_interface::display_certificate_non_const(std::ostream& out) {
        if(_d->status == StatusModel){
            ast_manager &m = m_ctx.get_manager();
            model_ref md = get_model();
            out << "(fixedpoint \n";
            model_smt2_pp(out, m, *md.get(), 0);
            out << ")\n";
        }
        else if(_d->status == StatusRefutation){
            out << "(derivation\n";
            // negation of the query is the last clause -- prove it
            hash_set<func_decl> locals;
            local_func_decls = &locals;
            print_proof(this,out,_d->cex.get_tree(),_d->cex.get_root());
            out << ")\n";
            out << "(model \n\"";
            ::model mod(m_ctx.get_manager());
            model orig_model = _d->cex.get_tree()->dualModel;
            for(unsigned i = 0; i < orig_model.num_consts(); i++){
                func_decl cnst = orig_model.get_const_decl(i);
                if (locals.find(cnst) == locals.end()) {
                    expr thing = orig_model.get_const_interp(cnst);
                    mod.register_decl(to_func_decl(cnst.raw()), to_expr(thing.raw()));
                }
            }
            for(unsigned i = 0; i < orig_model.num_funcs(); i++){
                func_decl cnst = orig_model.get_func_decl(i);
                if (locals.find(cnst) == locals.end()) {
                    func_interp thing = orig_model.get_func_interp(cnst);
                    ::func_interp *thing_raw = thing;
                    mod.register_decl(to_func_decl(cnst.raw()), thing_raw->copy());
                }
            }
            model_v2_pp(out,mod);
            out << "\")\n";
        }
    }

    expr_ref dl_interface::get_answer() {
        // FORK FIX #92 (g3 audit, see FORK_BUGFIXES.md): report the unsupported answer instead of returning null (AUDIT duality zbug round 2)
        // The null answer made fixedpoint.print_answer dereference a null
        // expression and crash.
        throw default_exception("duality does not support get_answer");
    }

    void dl_interface::cancel() {
#if 0
        if(_d && _d->ls)
            _d->ls->cancel();
#else
        // HACK: duality can't cancel at all times, we just exit here
        std::cout << "(error \"duality canceled\")\nunknown\n";
        abort();
#endif
    }

    void dl_interface::cleanup() {
    }

    void dl_interface::updt_params() {
    }

    model_ref dl_interface::get_model() {
        ast_manager &m = m_ctx.get_manager();
        model_ref md(alloc(::model, m));
        std::vector<RPFP::Node *> &nodes = _d->rpfp->nodes;
        expr_ref_vector conjs(m);
        for (unsigned i = 0; i < nodes.size(); ++i) {
            RPFP::Node *node = nodes[i];
            func_decl &pred = node->Name;
            expr_ref prop(m);
            prop = to_expr(node->Annotation.Formula);
            std::vector<expr> &params = node->Annotation.IndParams;
            expr_ref q(m);
            expr_ref_vector sig_vars(m);
            for (unsigned j = 0; j < params.size(); ++j)
                sig_vars.push_back(params[params.size()-j-1]); // TODO: why backwards?
            expr_abstract(m, 0, sig_vars.size(), sig_vars.c_ptr(), prop, q);
            if (params.empty()) {
                md->register_decl(pred, q);
            }
            else {
                ::func_interp* fi = alloc(::func_interp, m, params.size());
                fi->set_else(q);
                md->register_decl(pred, fi);
            }
        }
        return md;
    }

    // FORK FIX #93: record the proof of a background axiom and, by
    // and-elimination, of each of its conjuncts (depth first, in order).
    static void background_literal_proofs(ast_manager &m, ::expr *f, ::proof *p, obj_map< ::expr, ::proof*> &proofs, proof_ref_vector &pins) {
        proofs.insert(f, p);
        if (!m.is_and(f))
            return;
        for (unsigned k = 0; k < to_app(f)->get_num_args(); k++) {
            pins.push_back(m.mk_and_elim(p, k));
            background_literal_proofs(m, to_app(f)->get_arg(k), pins.back(), proofs, pins);
        }
    }

    static proof_ref extract_proof(dl_interface *d, RPFP *tree, RPFP::Node *root) {
        context &ctx = d->dd()->ctx;
        ast_manager &mgr = ctx.m();
        RPFP::Node &node = *root;
        RPFP::Edge &edge = *node.Outgoing;
        RPFP::Edge *orig_edge = edge.map;

        // first, prove the children (that are actually used)

        proof_ref_vector prems(mgr);
        ::vector<expr_ref_vector> substs;
        int orig_clause = d->dd()->map[orig_edge];
        expr &t = d->dd()->clauses[orig_clause];
        prems.push_back(mgr.mk_asserted(ctx.uncook(t)));

        substs.push_back(expr_ref_vector(mgr));

        // FORK FIX #93 (g3 audit, see FORK_BUGFIXES.md): a checkable hyper-resolve step (AUDIT duality zbug round 2)
        // The positions name the body literal each child proof resolves (a
        // child used twice resolves each of its literals), not the child's
        // index; the substitution is in variable-index order, as var_subst
        // in the proof checker reads it; and the conclusion is the original
        // clause head (false for the query, not @Fail).
        ::expr *raw_t = to_expr(t.raw());
        bool is_forall = is_quantifier(raw_t) && to_quantifier(raw_t)->is_forall();
        ::expr *raw_clause = is_forall ? to_quantifier(raw_t)->get_expr() : raw_t;
        vector<unsigned_vector> positions;
        bool shaped = witness_positions(d, raw_clause, positions) && positions.size() == edge.Children.size();

        svector<std::pair<unsigned, unsigned> > pos;
        for(unsigned i = 0; i < edge.Children.size(); i++){
            if(!tree->Empty(edge.Children[i])){
                proof_ref prem = extract_proof(d,tree,edge.Children[i]);
                unsigned_vector child_pos;
                if (shaped)
                    child_pos = positions[i];
                else
                    child_pos.push_back(i+1);
                for (unsigned k = 0; k < child_pos.size(); k++) {
                    pos.push_back(std::pair<unsigned,unsigned>(child_pos[k],0));
                    prems.push_back(prem);
                    substs.push_back(expr_ref_vector(mgr));
                }
            }
        }

        if (t.is_quantifier() && t.is_quantifier_forall()) {
            int bound = t.get_quantifier_num_bound();
            std::vector<sort> sorts;
            std::vector<symbol> names;
            hash_map<int,expr> subst;
            // FORK FIX #63 (g3 audit, see FORK_BUGFIXES.md): substitute the original rule's witness (AUDIT duality_01-F4)
            expr_ref_vector witness(mgr);
            bool has_witness = find_step_witness(d,tree,root,witness);
            expr_ref_vector by_decl(mgr);
            for(int j = 0; j < bound; j++){
                sort the_sort = t.get_quantifier_bound_sort(j);
                symbol name = t.get_quantifier_bound_name(j);
                expr skolem = ctx.constant(symbol(ctx,name),sort(ctx,the_sort));
                expr val = has_witness ? ctx.cook(witness.get(j)) : tree->Eval(&edge,skolem);
                by_decl.push_back(to_expr(val.raw()));
                expr local_skolem = tree->Localize(&edge,skolem);
                (*local_func_decls).insert(local_skolem.decl());
            }
            // FORK FIX #93: variable k is declaration bound-1-k
            for(int k = 0; k < bound; k++)
                substs[0].push_back(by_decl.get(bound-1-k));
        }

        // FORK FIX #93: a body literal that is a background axiom (or a
        // conjunct of one) is resolved with that axiom's proof, as g3's
        // backgroundLiteralProofs does, so the step has no open side condition.
        if (shaped) {
            expr_ref instance(raw_clause, mgr);
            if (!substs[0].empty()) {
                var_subst vs(mgr, false);
                vs(raw_clause, substs[0].size(), substs[0].c_ptr(), instance);
            }
            obj_map< ::expr, ::proof*> background;
            proof_ref_vector background_pins(mgr);
            for (unsigned i = 0; i < d->dd()->axioms.size(); i++) {
                ::expr *ax = to_expr(d->dd()->axioms[i].raw());
                background_pins.push_back(mgr.mk_asserted(ax));
                background_literal_proofs(mgr, ax, background_pins.back(), background, background_pins);
            }
            if (mgr.is_implies(instance)) {
                ::expr *ibody = to_app(instance)->get_arg(0);
                ptr_vector< ::expr> iatoms;
                if (mgr.is_and(ibody))
                    iatoms.append(to_app(ibody)->get_num_args(), to_app(ibody)->get_args());
                else
                    iatoms.push_back(ibody);
                uint_set predicate_positions;
                for (unsigned i = 0; i < positions.size(); i++)
                    for (unsigned k = 0; k < positions[i].size(); k++)
                        predicate_positions.insert(positions[i][k]);
                for (unsigned i = 0; i < iatoms.size(); i++) {
                    ::proof *bp;
                    if (!predicate_positions.contains(i + 1) && background.find(iatoms[i], bp)) {
                        pos.push_back(std::pair<unsigned,unsigned>(i + 1, 0));
                        prems.push_back(bp);
                        substs.push_back(expr_ref_vector(mgr));
                    }
                }
            }
        }

        func_decl f = node.Name;
        std::vector<expr> args;
        for(unsigned i = 0; i < edge.F.IndParams.size(); i++)
            args.push_back(tree->Eval(&edge,edge.F.IndParams[i]));
        expr conc = f(args);
        {
            // FORK FIX #93: conclude the original head
            ::expr *body;
            ptr_vector< ::expr> atoms;
            ::expr *head = witness_clause_shape(mgr, raw_clause, body, atoms);
            if (shaped && is_app(head) && to_app(head)->get_decl()->get_arity() <= args.size()) {
                ptr_vector< ::expr> hargs;
                for (unsigned i = 0; i < to_app(head)->get_decl()->get_arity(); i++)
                    hargs.push_back(to_expr(args[i].raw()));
                conc = ctx.cook(mgr.mk_app(to_app(head)->get_decl(), hargs.size(), hargs.c_ptr()));
            }
        }


        ::vector< ::proof *> pprems;
        for(unsigned i = 0; i < prems.size(); i++)
            pprems.push_back(prems[i].get());

        proof_ref res(mgr.mk_hyper_resolve(pprems.size(),&pprems[0], ctx.uncook(conc), pos, substs),mgr);
        return res;

    }

    proof_ref dl_interface::get_proof() {
        if(_d->status == StatusRefutation){
            hash_set<func_decl> locals;
            local_func_decls = &locals;
            return extract_proof(this,_d->cex.get_tree(),_d->cex.get_root());
        }
        else
            return proof_ref(m_ctx.get_manager());
    }
}
