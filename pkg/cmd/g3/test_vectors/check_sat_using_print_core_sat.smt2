(set-option :produce-unsat-cores true)(declare-const x Bool)(assert (! x :named a))(check-sat-using smt :print_unsat_core true)
