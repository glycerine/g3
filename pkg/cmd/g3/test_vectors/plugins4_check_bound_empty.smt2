(set-option :fixedpoint.engine datalog)
(set-option :fixedpoint.datalog.check_relation bound_relation)
(declare-rel p (Int))
(query p)
